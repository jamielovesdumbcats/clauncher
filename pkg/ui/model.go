package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
	"clauncher/pkg/ui/messages"
	"clauncher/pkg/ui/theme"

	"github.com/charmbracelet/bubbles/spinner"
	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// terminalWidth returns the current terminal width or a default of 80.
func (a *App) terminalWidth() int {
	if a.termWidth > 0 {
		return a.termWidth
	}
	return 80
}

// gpuTickMsg is sent every 2s to refresh GPU stats.
type gpuTickMsg struct{}

func gpuTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return gpuTickMsg{}
	})
}

// serversTickMsg is sent every minute to refresh running server list.
type serversTickMsg struct{}

func serversTickCmd() tea.Cmd {
	return tea.Tick(1*time.Minute, func(t time.Time) tea.Msg {
		return serversTickMsg{}
	})
}

// tickMsg is a simple message for spinning the spinner.
type tickMsg struct{}

// tickSpinner returns a command that ticks the spinner.
func tickSpinner() tea.Cmd {
	return func() tea.Msg {
		return tickMsg{}
	}
}

type ViewState int

const (
	ViewSelection ViewState = iota
	ViewDashboard
	ViewLaunchOptions
	ViewBenchmark
	ViewLaunchConfig
	ViewCatalog
	ViewKillServer
	ViewSearch
	ViewQuants
)

// App is the root model for the application.
type App struct {
	currentView ViewState
	theme       *theme.Theme
	termWidth   int // terminal width for gradient bars

	// Shared Context/State
	models        []model.Model
	selectedModel *model.Model
	runner        server.ProcessRunner

	// Internal state for transitions
	err error

	// Log state for dashboard
	logs []string

	// Model refresh state
	refreshing bool

	// Pending model selected, waiting for launch option
	pendingModel *model.Model

	// Cursor positions for arrow-key navigation
	cursorPos int // model or option index in selection/launch views

	// Catalog state
	catalog           []server.CatalogModel
	catalogCursor     int
	downloading       string
	downloadBusy      bool
	downloadCancel    context.CancelFunc
	downloadCancelled bool
	downloadProgress  string
	// placeholder

	// Running llama server detection
	runtimeServers []server.RunningLlamaProcess

	// Benchmark state
	benchmarkStore     *model.BenchmarkStore
	benchmarkResults   []model.BenchmarkResult
	benchmarking       bool
	benchmarkModelName string

	// Context for process management
	ctx      context.Context
	cancelFn context.CancelFunc

	// Launch config state
	portInput      textinput.Model
	ctxSizeInput   textinput.Model
	workDirInput   textinput.Model
	focusedInput   int // 0=port, 1=ctx, 2=dir
	pendingLaunch  model.LaunchOption
	ctxSizeWarning string

	// GPU info
	gpuStats server.GPUStats

	// Spinner for busy states
	spin spinner.Model
	// placeholder

	// Search state
	searchInput      textinput.Model
	searchResults    []server.SearchHFModel
	searchQuery      string
	repoFiles        []server.HFFile
	selectedRepo     string
	fileCursor       int
	fileScrollOffset int
	searchBusy       bool

	searchCursor int

	// Confirmation modal state
	modalShow     bool
	modalMessage  string
	modalCallback func() tea.Cmd
	termHeight    int // terminal height for modal overlay
}

// NewApp initializes a new application instance.
func NewApp(models []model.Model, runner server.ProcessRunner, runningServers []server.RunningLlamaProcess, catalog []server.CatalogModel) *App {
	ctx, cancel := context.WithCancel(context.Background())
	store := model.NewBenchmarkStore("")
	results, _ := store.Load()

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	portInput := textinput.New()
	portInput.Placeholder = "8081"
	portInput.SetValue("8081")

	ctxSizeInput := textinput.New()
	ctxSizeInput.Placeholder = "131072"
	ctxSizeInput.SetValue("131072")

	home, _ := os.UserHomeDir()
	workDirInput := textinput.New()
	workDirInput.Placeholder = home
	workDirInput.SetValue(home)

	searchInput := textinput.New()
	searchInput.Placeholder = "Search HuggingFace for GGUF models..."
	searchInput.Focus()

	return &App{
		currentView:      ViewSelection,
		theme:            theme.NewTheme(),
		models:           models,
		runner:           runner,
		logs:             []string{},
		ctx:              ctx,
		cancelFn:         cancel,
		benchmarkStore:   store,
		benchmarkResults: results,
		portInput:        portInput,
		ctxSizeInput:     ctxSizeInput,
		workDirInput:     workDirInput,
		focusedInput:     0,
		runtimeServers:   runningServers,
		gpuStats:         server.GetGPUStats(),
		catalog:          catalog,
		spin:             spin,
		searchInput:      searchInput,
		searchResults:    []server.SearchHFModel{},
		repoFiles:        []server.HFFile{},
	}
}

// Init starts the application.
func (a *App) Init() tea.Cmd {
	return tea.Batch(gpuTickCmd(), serversTickCmd())
}

// Update handles all incoming messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.termWidth = m.Width
		a.termHeight = m.Height
		return a, tea.Cmd(tea.ClearScreen)

	case tea.KeyMsg:
		if a.modalShow {
			if m.String() == "y" {
				a.modalShow = false
				if a.modalCallback != nil {
					return a, a.modalCallback()
				}
				return a, nil
			}
			if m.String() == "n" || m.String() == "esc" || m.String() == "b" {
				a.modalShow = false
				return a, nil
			}
			return a, nil
		}
		if a.currentView == ViewSearch {
			if m.String() == "ctrl+c" {
				return a, tea.Quit
			}
			if m.String() == "b" && a.downloadBusy {
				if a.downloadCancel != nil {
					a.downloadCancel()
				}
				a.downloadCancelled = true
				a.downloadBusy = false
				a.downloadCancel = nil
				a.downloading = ""
				a.currentView = ViewSelection
				a.err = nil
				return a, nil
			}
			if m.String() == "b" && !a.searchInput.Focused() {
				return a, a.goBack()
			}
			if m.String() == "esc" {
				if a.downloadBusy {
					a.currentView = ViewSelection
					a.err = nil
					return a, nil
				}
				return a, a.goBack()
			}
			if m.String() == "enter" && a.searchInput.Focused() && a.searchInput.Value() != "" && !a.searchBusy {
				a.searchQuery = a.searchInput.Value()
				a.searchBusy = true
				a.searchResults = nil
				a.searchCursor = 0
				a.searchInput.Blur()
				return a, tea.Batch(
					func() tea.Msg {
						results, err := server.SearchHFModels(a.searchQuery)
						return messages.SearchCompleteMsg{Results: results, Error: err}
					},
					tickSpinner(),
				)
			}
			if m.String() == "up" && len(a.searchResults) > 0 && a.searchCursor > 0 {
				a.searchCursor--
				return a, nil
			}
			if m.String() == "down" && len(a.searchResults) > 0 && a.searchCursor < len(a.searchResults)-1 {
				a.searchCursor++
				return a, nil
			}
			if m.String() == "enter" && len(a.searchResults) > 0 && a.searchCursor < len(a.searchResults) && !a.searchBusy && !a.downloadBusy {
				a.selectedRepo = a.searchResults[a.searchCursor].ModelID
				a.repoFiles = nil
				a.fileCursor = 0
				a.fileScrollOffset = 0
				a.currentView = ViewQuants
				a.searchBusy = true
				return a, tea.Batch(
					func() tea.Msg {
						files, err := server.GetHFModelFiles(a.selectedRepo)
						return messages.FilesCompleteMsg{Files: files, Error: err}
					},
					tickSpinner(),
				)
			}
			var cmd tea.Cmd
			if !a.searchInput.Focused() {
				a.searchInput, cmd = a.searchInput.Update(msg)
			} else {
				// Don't send Enter to textinput when results are shown
				if m.Type != tea.KeyEnter || len(a.searchResults) == 0 {
					a.searchInput, cmd = a.searchInput.Update(msg)
				}
			}
			if cmd != nil {
				return a, cmd
			}
			return a, nil
		}
		if a.currentView == ViewQuants {
			if m.String() == "ctrl+c" {
				return a, tea.Quit
			}
			if m.String() == "b" || m.String() == "esc" {
				return a, a.goBack()
			}
			if m.String() == "up" && a.fileCursor > 0 {
				a.fileCursor--
				return a, nil
			}
			if m.String() == "down" && a.fileCursor < len(a.repoFiles)-1 {
				a.fileCursor++
				return a, nil
			}
			if m.String() == "enter" && len(a.repoFiles) > 0 && a.fileCursor < len(a.repoFiles) {
				quant := a.repoFiles[a.fileCursor]
				return a, func() tea.Msg {
					hfRepo := a.selectedRepo + ":" + quant.Filename
					displayName := strings.ReplaceAll(a.selectedRepo, "/", " - ")
					displayName = displayName + " (" + quant.Filename + ")"
					sizeGB := float64(quant.Size) / (1024 * 1024 * 1024)
					return messages.ModelAddedToCatalogMsg{
						Model: server.CatalogModel{
							HFRepo:      hfRepo,
							DisplayName: displayName,
							SizeGB:      sizeGB,
							Tags:        a.searchResults[a.searchCursor].Tags,
						},
					}
				}
			}
			return a, nil
		}
		switch m.String() {
		case "ctrl+c", "q":
			if a.currentView == ViewDashboard {
				// Stop any running process before quitting
				if a.runner.Status().Status == model.StatusRunning {
					a.runner.Stop()
				}
			}
			return a, tea.Quit
		case "s":
			if a.currentView == ViewDashboard {
				return a, a.toggleProcess()
			}
			if a.currentView == ViewSelection {
				a.searchInput.SetValue("")
				a.searchResults = nil
				a.searchCursor = 0
				a.currentView = ViewSearch
				a.searchInput.Focus()
				return a, nil
			}
		case "b":
			// Cancel download if in progress, then go back
			if a.currentView == ViewCatalog && a.downloadBusy {
				if a.downloadCancel != nil {
					a.downloadCancel()
				}
				a.downloadCancelled = true
				a.downloadBusy = false
				a.downloadCancel = nil
				a.downloading = ""
				a.currentView = ViewSelection
				a.err = nil
				return a, nil
			}
			if a.currentView == ViewDashboard || a.currentView == ViewLaunchOptions || a.currentView == ViewBenchmark || a.currentView == ViewLaunchConfig || a.currentView == ViewCatalog || a.currentView == ViewKillServer || a.currentView == ViewQuants {
				return a, a.goBack()
			}
		case "esc":
			// Navigate away, keep download running in background
			if a.currentView == ViewCatalog && a.downloadBusy {
				a.currentView = ViewSelection
				a.err = nil
				return a, nil
			}
			if a.currentView == ViewDashboard || a.currentView == ViewLaunchOptions || a.currentView == ViewBenchmark || a.currentView == ViewLaunchConfig || a.currentView == ViewKillServer || a.currentView == ViewSearch || a.currentView == ViewQuants {
				return a, a.goBack()
			}
			if a.currentView == ViewCatalog {
				return a, a.goBack()
			}
		case "m":
			// Navigate to benchmark results or run benchmark
			if a.currentView == ViewSelection {
				a.currentView = ViewBenchmark
				return a, nil
			}
			if a.currentView == ViewBenchmark && a.pendingModel != nil && !a.benchmarking {
				a.benchmarking = true
				a.benchmarkModelName = a.pendingModel.Name
				return a, tea.Batch(a.runBenchmark(*a.pendingModel), tickSpinner())
			}
		case "d":
			// Navigate to catalog to download models
			if a.currentView == ViewSelection {
				a.currentView = ViewCatalog
				return a, nil
			}
		case "c":
			// Clear benchmark results
			if a.currentView == ViewBenchmark {
				if err := a.benchmarkStore.Clear(); err != nil {
					a.err = fmt.Errorf("failed to clear benchmarks: %w", err)
				} else {
					a.benchmarkResults = nil
				}
				return a, nil
			}
		case "r":
			// Refresh local models list
			if a.currentView == ViewSelection && !a.refreshing {
				return a, a.refreshModels()
			}
		case "x":
			// Delete selected model
			if a.currentView == ViewSelection && a.cursorPos < len(a.models) {
				return a, a.deleteModel(a.cursorPos)
			}
		case "k":
			// Kill running llama servers
			if a.currentView == ViewSelection || a.currentView == ViewLaunchOptions || a.currentView == ViewDashboard || a.currentView == ViewBenchmark || a.currentView == ViewCatalog || a.currentView == ViewLaunchConfig {
				if len(a.runtimeServers) == 0 {
					a.err = fmt.Errorf("no running servers to kill")
					return a, nil
				}
				a.cursorPos = 0
				a.currentView = ViewKillServer
				return a, nil
			}
			if a.currentView == ViewKillServer {
				return a, a.killSelectedServer()
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Select model by number (in selection view)
			if a.currentView == ViewSelection {
				idx := int(m.String()[0] - '1') // Convert '1'-'9' to 0-8
				if idx >= 0 && idx < len(a.models) {
					return a, a.selectModel(idx)
				}
			}
			// Select launch option (in launch options view)
			if a.currentView == ViewLaunchOptions {
				option := model.LaunchLlamaServer
				if m.String() == "2" {
					option = model.LaunchLlamaCLI
				} else if m.String() == "3" {
					option = model.LaunchClaudeCode
				} else if m.String() == "4" {
					option = model.LaunchOpencode
				} else if m.String() == "5" {
					option = model.LaunchCrush
				}

				// Option 6: run benchmark
				if m.String() == "6" {
					if a.pendingModel != nil {
						a.currentView = ViewBenchmark
						a.benchmarking = true
						a.benchmarkModelName = a.pendingModel.Name
						return a, a.runBenchmark(*a.pendingModel)
					}
					return a, nil
				}

				// Options 3-5 show a config prompt first
				if option == model.LaunchClaudeCode || option == model.LaunchOpencode || option == model.LaunchCrush {
					a.pendingLaunch = option
					a.currentView = ViewLaunchConfig
					a.focusedInput = 0
					a.ctxSizeWarning = ""
					cmds := []tea.Cmd{
						a.portInput.Focus(),
					}
					return a, tea.Batch(cmds...)
				}
				return a, func() tea.Msg {
					return messages.LaunchOptionSelectedMsg{
						Option: option,
						Model:  *a.pendingModel,
					}
				}
			}
		case "up":
			// Navigate up in selection or launch options views
			if a.currentView == ViewSelection && a.cursorPos > 0 {
				a.cursorPos--
			} else if a.currentView == ViewLaunchOptions && a.cursorPos > 0 {
				a.cursorPos--
			} else if a.currentView == ViewCatalog && a.catalogCursor > 0 {
				a.catalogCursor--
			} else if a.currentView == ViewKillServer && a.cursorPos > 0 {
				a.cursorPos--
			}
			return a, nil
		case "down":
			// Navigate down in selection or launch options views
			if a.currentView == ViewSelection && a.cursorPos < len(a.models)-1 {
				a.cursorPos++
			} else if a.currentView == ViewLaunchOptions && a.cursorPos < 5 {
				a.cursorPos++
			} else if a.currentView == ViewCatalog && a.catalogCursor < len(a.catalog)-1 {
				a.catalogCursor++
			} else if a.currentView == ViewKillServer && a.cursorPos < len(a.runtimeServers)-1 {
				a.cursorPos++
			}
			return a, nil
		case "enter":
			if a.currentView == ViewSelection && a.cursorPos < len(a.models) {
				return a, a.selectModel(a.cursorPos)
			}
			if a.currentView == ViewLaunchOptions {
				switch a.cursorPos {
				case 0: // Llama Server
					a.selectedModel = a.pendingModel
					a.currentView = ViewDashboard
					a.logs = []string{}
					return a, a.startProcess(*a.pendingModel)
				case 1: // Llama CLI
					return a, tea.Batch(a.launchLlamaCLI(*a.pendingModel), func() tea.Msg { return tea.ClearScreen() })
				case 2: // Claude Code
					return a, tea.Batch(a.launchClaudeCodeWithConfig(*a.pendingModel, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
				case 3: // Opencode
					return a, tea.Batch(a.launchOpencodeWithConfig(*a.pendingModel, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
				case 4: // Crush
					return a, tea.Batch(a.launchCrushWithConfig(*a.pendingModel, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
				case 5: // Benchmark
					a.currentView = ViewBenchmark
					a.benchmarking = true
					a.benchmarkModelName = a.pendingModel.Name
					return a, a.runBenchmark(*a.pendingModel)
				}
			}
			if a.currentView == ViewCatalog && a.catalogCursor < len(a.catalog) {
				m := a.catalog[a.catalogCursor]
				if server.IsModelDownloaded(m.HFRepo) {
					a.err = fmt.Errorf("%s is already downloaded", m.DisplayName)
					return a, nil
				}
				a.downloading = m.DisplayName
				a.downloadBusy = true
				a.downloadProgress = ""
				timeout := 30 * time.Minute
				if m.SizeGB > 30 {
					timeout = 60 * time.Minute
				}
				ctx, cancel := context.WithTimeout(a.ctx, timeout)
				a.downloadCancel = cancel
				return a, tea.Batch(
					func() tea.Msg {
						defer cancel()
						err := server.DownloadModel(ctx, m.HFRepo)
						return messages.DownloadCompleteMsg{Model: m.DisplayName, Error: err}
					},
					tickSpinner(),
				)
			}
			return a, nil
		}

		// Handle text input in launch config view
		if a.currentView == ViewLaunchConfig {
			if m.String() == "tab" {
				a.focusedInput = (a.focusedInput + 1) % 3
				a.portInput.Blur()
				a.ctxSizeInput.Blur()
				a.workDirInput.Blur()
				switch a.focusedInput {
				case 0:
					a.portInput.Focus()
				case 1:
					a.ctxSizeInput.Focus()
				case 2:
					a.workDirInput.Focus()
				}
				return a, nil
			}
			if m.String() == "shift+tab" {
				a.focusedInput--
				if a.focusedInput < 0 {
					a.focusedInput = 2
				}
				a.portInput.Blur()
				a.ctxSizeInput.Blur()
				a.workDirInput.Blur()
				switch a.focusedInput {
				case 0:
					a.portInput.Focus()
				case 1:
					a.ctxSizeInput.Focus()
				case 2:
					a.workDirInput.Focus()
				}
				return a, nil
			}
			if m.String() == "enter" {
				// Validate and proceed with launch
				port := a.portInput.Value()
				if port == "" {
					port = "8081"
				}
				ctxSize := a.ctxSizeInput.Value()
				if ctxSize == "" {
					ctxSize = "131072"
				}
				workDir := a.workDirInput.Value()
				if workDir == "" {
					home, _ := os.UserHomeDir()
					workDir = home
				}

				// Context size warning
				if ctxSizeInt, _ := strconv.Atoi(ctxSize); ctxSizeInt > 0 && ctxSizeInt < 4096 {
					a.ctxSizeWarning = "Warning: context size < 4096 may cause issues with Opencode/Crush. Press Enter again to proceed or 'b' to change."
					return a, nil
				}
				a.ctxSizeWarning = ""

				// Proceed with the launch
				a.currentView = ViewSelection
				return a, func() tea.Msg {
					return messages.LaunchOptionSelectedMsg{
						Option: a.pendingLaunch,
						Model:  *a.pendingModel,
					}
				}
			}

			// Route key only to the focused input
			var cmd tea.Cmd
			switch a.focusedInput {
			case 0:
				a.portInput, cmd = a.portInput.Update(msg)
			case 1:
				a.ctxSizeInput, cmd = a.ctxSizeInput.Update(msg)
			case 2:
				a.workDirInput, cmd = a.workDirInput.Update(msg)
			}
			return a, cmd
		}
	case gpuTickMsg:
		a.gpuStats = server.GetGPUStats()
		return a, gpuTickCmd()

	case serversTickMsg:
		if procs, err := server.FindRunningLlamaServers(); err == nil {
			a.runtimeServers = procs
		}
		return a, serversTickCmd()

	case tickMsg:
		// Tick the spinner during busy states
		if a.benchmarking || a.downloadBusy || a.searchBusy {
			var cmd tea.Cmd
			a.spin, cmd = a.spin.Update(tickMsg{})
			return a, tea.Batch(cmd, tickSpinner())
		}
		return a, nil

	case messages.LogMsg:
		a.logs = append(a.logs, m.Line)
		// Keep log buffer reasonable
		if len(a.logs) > 100 {
			a.logs = a.logs[1:]
		}
		return a, nil

	case messages.StatusUpdateMsg:
		if m.Error != nil {
			a.err = m.Error
		}
		// Start the status tick loop if process is running
		if m.Status == model.StatusRunning {
			return a, tick()
		}
		return a, nil

	case messages.ErrorMsg:
		a.err = m.Err
		return a, nil

	case messages.StatusTickMsg:
		// Check for status changes
		currentStatus := a.runner.Status().Status
		if currentStatus == model.StatusCrashed {
			if info := a.runner.Status(); info.Error != nil {
				a.err = info.Error
			}
		}
		// Continue the tick loop if still in dashboard
		if a.currentView == ViewDashboard {
			return a, tick()
		}
		return a, nil

	case messages.ModelsRefreshedMsg:
		a.refreshing = false
		if m.Error != nil {
			a.err = m.Error
			return a, nil
		}
		a.models = m.Models
		return a, nil

	case messages.LaunchOptionSelectedMsg:
		switch m.Option {
		case model.LaunchLlamaServer:
			// Use existing dashboard flow
			a.selectedModel = &m.Model
			a.currentView = ViewDashboard
			a.logs = []string{}
			return a, a.startProcess(m.Model)
		case model.LaunchLlamaCLI:
			// Launch CLI in new terminal
			return a, tea.Batch(a.launchLlamaCLI(m.Model), func() tea.Msg { return tea.ClearScreen() })
		case model.LaunchClaudeCode:
			// Launch Claude Code with local model using config
			return a, tea.Batch(a.launchClaudeCodeWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
		case model.LaunchOpencode:
			// Launch Opencode with local model using config
			return a, tea.Batch(a.launchOpencodeWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
		case model.LaunchCrush:
			// Launch Crush with local model using config
			return a, tea.Batch(a.launchCrushWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value()), func() tea.Msg { return tea.ClearScreen() })
		}
		return a, nil

	case messages.BenchmarkCompleteMsg:
		a.benchmarking = false
		if m.Error != nil {
			a.err = m.Error
			return a, nil
		}
		if m.Result != nil {
			_ = a.benchmarkStore.Add(*m.Result)
			a.benchmarkResults = append(a.benchmarkResults, *m.Result)
		}
		return a, nil

	case messages.DownloadCompleteMsg:
		a.downloadCancel = nil
		if a.downloadCancelled {
			a.downloadCancelled = false
			a.downloading = ""
			a.downloadBusy = false
			return a, nil
		}
		if m.Error != nil {
			a.downloadBusy = false
			a.err = fmt.Errorf("download failed: %w", m.Error)
			return a, nil
		}
		a.downloading = ""
		a.downloadBusy = false
		// Refresh model list after download
		return a, tea.Batch(a.refreshModels(), func() tea.Msg {
			return messages.SuccessMsg{Message: fmt.Sprintf("Downloaded %s successfully", m.Model)}
		})

	case messages.SearchCompleteMsg:
		a.searchBusy = false
		if m.Error != nil {
			a.err = m.Error
			return a, nil
		}
		a.searchResults = m.Results
		a.searchInput.Blur()
		return a, nil

	case messages.FilesCompleteMsg:
		a.searchBusy = false
		if m.Error != nil {
			a.err = fmt.Errorf("failed to fetch repo files: %w", m.Error)
			return a, nil
		}
		a.repoFiles = m.Files
		return a, nil

	case messages.ModelAddedToCatalogMsg:
		if m.Error != nil {
			a.err = m.Error
			return a, nil
		}
		a.catalog = append(a.catalog, m.Model)
		if err := server.SaveCatalog(a.catalog); err != nil {
			a.err = fmt.Errorf("failed to save catalog: %w", err)
			return a, nil
		}
		a.currentView = ViewCatalog

	case messages.ModelDeletedMsg:
		if m.Error != nil {
			a.err = fmt.Errorf("delete failed: %w", m.Error)
			return a, nil
		}
		return a, tea.Batch(a.refreshModels(), func() tea.Msg {
			return messages.SuccessMsg{Message: fmt.Sprintf("Deleted %s", m.Model)}
		})

	case messages.SuccessMsg:
		a.downloadProgress = m.Message
		return a, nil
		a.catalogCursor = len(a.catalog) - 1
		return a, nil
	}

	return a, nil
}

// toggleProcess handles the start/stop logic in the dashboard
func (a *App) toggleProcess() tea.Cmd {
	if a.selectedModel == nil {
		return nil
	}

	status := a.runner.Status()
	if status.Status == model.StatusRunning {
		// Stop the process
		err := a.runner.Stop()
		if err != nil {
			return func() tea.Msg {
				return messages.ErrorMsg{Err: err}
			}
		}
		// Clear any error state that might have been set
		a.runner.ClearError()
		a.err = nil
		return func() tea.Msg {
			return messages.StatusUpdateMsg{Status: model.StatusStopped}
		}
	}
	// Start the process
	return a.startProcess(*a.selectedModel)
}

// goBack returns to the selection view
func (a *App) goBack() tea.Cmd {
	if a.currentView == ViewLaunchConfig {
		a.currentView = ViewLaunchOptions
	} else if a.currentView == ViewLaunchOptions {
		a.currentView = ViewSelection
		a.pendingModel = nil
	} else if a.currentView == ViewDashboard {
		a.currentView = ViewSelection
		a.selectedModel = nil
	} else if a.currentView == ViewBenchmark || a.currentView == ViewCatalog || a.currentView == ViewKillServer || a.currentView == ViewSearch || a.currentView == ViewQuants {
		if a.currentView == ViewQuants {
			a.currentView = ViewSearch
			a.fileCursor = 0
			a.fileScrollOffset = 0
			a.searchInput.Focus()
			return nil
		}
		a.currentView = ViewSelection
	}
	a.err = nil // Clear any error state when returning to selection
	return nil
}

// killSelectedServer shows a confirmation modal before killing the server.
func (a *App) killSelectedServer() tea.Cmd {
	if a.cursorPos < 0 || a.cursorPos >= len(a.runtimeServers) {
		a.err = fmt.Errorf("invalid server selection")
		return nil
	}

	target := a.runtimeServers[a.cursorPos]
	a.modalShow = true
	a.modalMessage = fmt.Sprintf("Kill PID %d?", target.PID)
	a.modalCallback = func() tea.Cmd {
		a.modalShow = false
		if err := server.KillLlamaServer(target.PID); err != nil {
			a.err = fmt.Errorf("failed to kill PID %d: %w", target.PID, err)
			return nil
		}

		a.runtimeServers = append(a.runtimeServers[:a.cursorPos], a.runtimeServers[a.cursorPos+1:]...)
		a.cursorPos = 0

		if len(a.runtimeServers) == 0 {
			a.currentView = ViewSelection
			a.err = nil
			return a.refreshModels()
		}

		a.err = nil
		return a.refreshModels()
	}
	return nil
}

// refreshModels triggers a refresh of the local models list
func (a *App) refreshModels() tea.Cmd {
	a.refreshing = true
	a.err = nil // Clear any previous error
	return func() tea.Msg {
		models, err := server.ListLocalModels()
		return messages.ModelsRefreshedMsg{Models: models, Error: err}
	}
}

// deleteModel shows a confirmation modal before removing the model at idx.
func (a *App) deleteModel(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.models) {
		a.err = fmt.Errorf("invalid model selection")
		return nil
	}

	m := a.models[idx]
	a.modalShow = true
	a.modalMessage = fmt.Sprintf("Delete %s?", m.Name)
	a.modalCallback = func() tea.Cmd {
		a.modalShow = false
		repoName := m.Config["model_name"]
		displayName := m.Name
		return func() tea.Msg {
			err := server.DeleteModel(repoName)
			return messages.ModelDeletedMsg{Model: displayName, Error: err}
		}
	}
	return nil
}

// View renders the current application state.
func (a *App) View() string {
	if a.err != nil {
		return a.theme.Error.Render(fmt.Sprintf("Error: %v\n\nPress 'b' to go back", a.err))
	}

	viewContent := ""
	switch a.currentView {
	case ViewSelection:
		viewContent = a.renderSelectionView()
	case ViewLaunchOptions:
		viewContent = a.renderLaunchOptionsView()
	case ViewDashboard:
		viewContent = a.renderDashboardView()
	case ViewBenchmark:
		viewContent = a.renderBenchmarkView()
	case ViewLaunchConfig:
		viewContent = a.renderLaunchConfigView()
	case ViewCatalog:
		viewContent = a.renderCatalogView()
	case ViewKillServer:
		viewContent = a.renderKillServerView()
	case ViewSearch:
		viewContent = a.renderSearchView()
	case ViewQuants:
		viewContent = a.renderQuantsView()
	default:
		return "Unknown View"
	}

	if a.modalShow {
		return a.renderModal()
	}
	return a.theme.GradientBar(a.terminalWidth()) + "\n" + viewContent
}

// renderModal renders a centered confirmation modal overlay with gradient bars.
func (a *App) renderModal() string {
	// Consistent header bar at top, matching other views
	header := a.theme.Header.Render("Confirmation")

	modalContent := fmt.Sprintf("  %s\n\n  %s %s\n  %s %s",
		a.modalMessage,
		a.theme.Success.Render("[y]"),
		a.theme.Primary.Render("Yes"),
		a.theme.Faint.Render("[n]"),
		a.theme.Faint.Render("No"),
	)

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorOrange).
		Padding(1, 3).
		Width(60).
		Align(lipgloss.Center).
		Render(modalContent)

	// Fixed height: 1 (content) + 2 (border) = 8 lines for modalBox
	boxHeight := 8
	// Total with padding: 12 lines
	totalHeight := boxHeight + 4

	width := a.termWidth
	height := a.termHeight
	if height <= 0 {
		height = 24
	}

	gradient := a.theme.GradientBar(a.terminalWidth())
	placed := lipgloss.Place(width, totalHeight, lipgloss.Center, lipgloss.Center, modalBox)
	return gradient + "\n" + header + "\n" + placed + "\n" + gradient
}

// renderGPUStats renders the GPU stats panel. Returns empty string if no GPU detected.
func renderGPUStats(a *App) string {
	if a.gpuStats.UsageText == "" && a.gpuStats.Temperature == 0 {
		return ""
	}

	if a.gpuStats.UsageText != "" {
		gpuContent := a.theme.PanelHeader("GPU Stats", a.theme.PanelTitleCyan) + a.theme.Faint.Render("\n"+a.gpuStats.UsageText)
		return a.theme.PanelCyan.Render(gpuContent)
	}

	// Fixed-width values: 5 chars max (e.g., "  99°C", " 100%")
	tempVal := fmt.Sprintf("%4.0f°C", a.gpuStats.Temperature)
	compVal := fmt.Sprintf("%4.0f%%", a.gpuStats.GPUUsage)
	memVal := fmt.Sprintf("%4.0f%%", a.gpuStats.MemoryUsage)

	// Color code utilization
	compStyle := a.theme.Faint
	if a.gpuStats.GPUUsage > 80 {
		compStyle = a.theme.Warning
	} else if a.gpuStats.GPUUsage > 40 {
		compStyle = a.theme.Info
	}

	memStyle := a.theme.Faint
	if a.gpuStats.MemoryUsage > 85 {
		memStyle = a.theme.Warning
	} else if a.gpuStats.MemoryUsage > 50 {
		memStyle = a.theme.Info
	}

	stats := fmt.Sprintf("  Temp:    %s\n  Compute: %s\n  VRAM:    %s",
		a.theme.Faint.Render(tempVal),
		compStyle.Render(compVal),
		memStyle.Render(memVal))

	gpuContent := a.theme.PanelHeader("GPU Stats", a.theme.PanelTitleCyan) + "\n" + stats
	return a.theme.PanelCyan.Render(gpuContent)
}

// renderServersStats renders the running servers panel.
func renderServersStats(a *App) string {
	content := ""
	if len(a.runtimeServers) == 0 {
		content = "  " + a.theme.Faint.Render("No servers running")
	} else {
		for i, p := range a.runtimeServers {
			line := fmt.Sprintf("PID %d", p.PID)
			if p.Port > 0 {
				line += fmt.Sprintf(" :%d", p.Port)
			}
			if p.Type != "" && p.Type != "unknown" {
				line += fmt.Sprintf(" (%s)", p.Type)
			}
			if i == 0 {
				content += a.theme.Warning.Render(line)
			} else {
				content += "\n" + a.theme.Warning.Render(line)
			}
		}
	}

	srvContent := a.theme.PanelHeader("Servers", a.theme.PanelTitleYellow) + "\n" + content
	return a.theme.PanelYellow.Render(srvContent)
}

// renderStatusPanes renders GPU and Servers panels side by side.
func renderStatusPanes(a *App) string {
	gpu := renderGPUStats(a)
	srv := renderServersStats(a)

	return lipgloss.JoinHorizontal(lipgloss.Top, gpu, srv) + "\n"
}

// renderSelectionView renders the model selection UI.
func (a *App) renderSelectionView() string {
	var s strings.Builder

	// ASCII Art header
	banner := `
   ╔═══════════════════════════════════╗
   ║   ████████╗███████╗██████╗ ████   ║
   ║   ╚══██╔══╝██╔════╝██╔══██╗███   ║
   ║      ██║   █████╗  ██████╔╝██    ║
   ║      ██║   ██╔══╝  ██╔══██╗██    ║
   ║      ██║   ███████╗██║  ██║██    ║
   ║      ╚═╝   ╚══════╝╚═╝  ╚═╝╝╝    ║
   ╚═══════════════════════════════════╝`
	s.WriteString(a.theme.Banner.Render(banner))
	s.WriteString("\n\n")

	// GPU + Servers status panels
	s.WriteString(renderStatusPanes(a))

	// Loading indicator
	if a.refreshing {
		s.WriteString(a.theme.Info.Render("  ↻ Refreshing model list...\n\n"))
	}

	// Download progress indicator
	if a.downloadBusy {
		s.WriteString(a.theme.Warning.Render(fmt.Sprintf("  %s Downloading: %s\n", a.spin.View(), a.downloading)))
	}

	// Models panel
	if len(a.models) == 0 && !a.refreshing {
		s.WriteString(a.theme.Faint.Render("  No local models found.\n"))
		s.WriteString(a.theme.Faint.Render("  Press " + a.theme.Key.Render("d") + " to browse catalog and download models.\n\n"))
	} else if len(a.models) > 0 {
		modelLines := ""
		for i, m := range a.models {
			if i == a.cursorPos {
				modelLines += fmt.Sprintf(" %s %d. %s\n", a.theme.PanelTitleMagenta.Render("▸"), i+1, m.Name)
			} else {
				modelLines += fmt.Sprintf("  %d. %s\n", i+1, m.Name)
			}
		}
		modelContent := a.theme.PanelHeader("Models", a.theme.PanelTitleMagenta) + modelLines
		modelsPanel := a.theme.PanelMagenta.Render(modelContent)
		s.WriteString(modelsPanel)
	}

	// Status bar footer
	hints := []string{"↑↓ Navigate", "Enter Select", "s Search", "d Catalog", "r Refresh", "k Kill", "m Bench", "x Delete", "q Quit"}
	s.WriteString(a.theme.StatusBar("Selection", hints, a.terminalWidth()))

	return s.String()
}

// renderLaunchOptionsView renders the launch options UI.
func (a *App) renderLaunchOptionsView() string {
	s := a.theme.Header.Render("Launch Options") + "\n\n"
	s += a.theme.Primary.Render(fmt.Sprintf("Model: %s\n\n", a.pendingModel.Name))
	s += renderStatusPanes(a)

	options := []struct {
		label string
		style func(strs ...string) string
		desc  string
	}{
		{"Launch Llama Server", a.theme.Success.Render, "open in browser"},
		{"Launch Llama CLI", a.theme.Secondary.Render, "new terminal"},
		{"Launch Claude Code", a.theme.Info.Render, "with local model"},
		{"Launch Opencode", a.theme.Info.Render, "with local model"},
		{"Launch Crush", a.theme.Info.Render, "with local model"},
		{"Run Benchmark", a.theme.Info.Render, "for this model"},
	}

	optionLines := ""
	for i, opt := range options {
		prefix := a.theme.Faint.Render(" ")
		if i == a.cursorPos {
			prefix = a.theme.Success.Render("▸")
		}
		optionLines += fmt.Sprintf("  %s %d. %s (%s)\n", prefix, i+1, opt.style(opt.label), opt.desc)
	}

	s += a.theme.PanelBlue.Render(a.theme.PanelTitle.Render("Options") + "\n" + optionLines)
	hints := []string{"↑↓ Navigate", "Enter Launch", "1-6 Quick", "b Back"}
	s += a.theme.StatusBar("Launch Options", hints, a.terminalWidth())
	return s
}

// renderDashboardView renders the main dashboard UI.
func (a *App) renderDashboardView() string {
	status := a.runner.Status().Status

	// Header with status indicator
	statusIndicator := "●"
	switch status {
	case model.StatusRunning:
		statusIndicator = "●"
	case model.StatusStarting:
		statusIndicator = "◐"
	case model.StatusStopped:
		statusIndicator = "○"
	case model.StatusCrashed:
		statusIndicator = "✕"
	}

	header := fmt.Sprintf("[%s] %s - %s", statusIndicator, status, a.selectedModel.Name)
	styledHeader := a.theme.Header.Render(header)

	// GPU stats
	gpuLine := renderStatusPanes(a)

	// Construct log area
	logContent := ""
	if len(a.logs) == 0 {
		if status == model.StatusStarting {
			logContent = "Starting process..."
		} else {
			logContent = "No logs yet. Press 's' to start the process."
		}
	} else {
		for _, line := range a.logs {
			logContent += line + "\n"
		}
	}

	// Use a bordered box for logs
	logBox := a.theme.PanelBlue.Render(logContent)

	hints := []string{"s Toggle", "b Back", "q Quit"}
	return fmt.Sprintf("%s%s\n\n%s%s", styledHeader, gpuLine, logBox, a.theme.StatusBar("Dashboard", hints, a.terminalWidth()))
}

// selectModel handles the transition from selection to launch options
func (a *App) selectModel(idx int) tea.Cmd {
	a.pendingModel = &a.models[idx]
	a.cursorPos = 0
	a.currentView = ViewLaunchOptions
	return nil
}

// startProcess starts the process and returns a command that reads logs.
func (a *App) startProcess(m model.Model) tea.Cmd {
	return func() tea.Msg {
		// Start the process
		logChan, err := a.runner.Start(a.ctx, m)
		if err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start process: %w", err)}
		}

		// Start reading logs in a goroutine
		go func() {
			for line := range logChan {
				a.logs = append(a.logs, line)
				if len(a.logs) > 100 {
					a.logs = a.logs[1:]
				}
			}
			// Channel closed - process exited
		}()

		// Start status ticker
		return messages.StatusUpdateMsg{Status: model.StatusRunning}
	}
}

// tickCmd returns a command that sends a StatusTickMsg
func tickCmd() tea.Msg {
	return messages.StatusTickMsg{}
}

// tick returns a command that sends a StatusTickMsg after a short delay
func tick() tea.Cmd {
	return func() tea.Msg {
		return messages.StatusTickMsg{}
	}
}

// launchLlamaCLI launches the llama CLI in a new terminal window
func (a *App) launchLlamaCLI(m model.Model) tea.Cmd {
	return func() tea.Msg {
		script := fmt.Sprintf("llama cli -hf %s && read", m.Config["model_name"])
		return spawnInTerminal(script)
	}
}

// spawnInTerminal tries to run a shell command in a terminal emulator.
// Falls back to background execution if no emulator is found.
func spawnInTerminal(script string) tea.Msg {
	fmt.Fprintf(os.Stderr, "[clauncher] spawnInTerminal: script=%q\n", script)

	switch runtime.GOOS {
	case "linux":
		type terminalSpec struct {
			name string
			args []string
		}
		terminals := []terminalSpec{
			{"gnome-terminal", []string{"--", "sh", "-c", script}},
			{"konsole", []string{"-e", "sh", "-c", script}},
			{"xterm", []string{"-e", "sh", "-c", script}},
			{"terminator", []string{"-x", "sh", "-c", script}},
			{"foot", []string{"-e", "sh", "-c", script}},
			{"kitty", []string{"--", "sh", "-c", script}},
			{"alacritty", []string{"-e", "sh", "-c", script}},
		}
		for _, term := range terminals {
			if path, err := exec.LookPath(term.name); err == nil {
				fmt.Fprintf(os.Stderr, "[clauncher] spawnInTerminal: found terminal %s at %s\n", term.name, path)
				cmd := exec.Command(path, term.args...)
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				if err := cmd.Start(); err == nil {
					cmd.Process.Release()
					return messages.StatusUpdateMsg{Status: model.StatusStopped}
				}
				fmt.Fprintf(os.Stderr, "[clauncher] spawnInTerminal: failed to start %s: %v\n", term.name, err)
			}
		}
		// Fallback: run detached in background
		fmt.Fprintf(os.Stderr, "[clauncher] spawnInTerminal: no terminal emulator found, falling back to background execution\n")
		cmd := exec.Command("sh", "-c", script+" & disown")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("no terminal emulator found and background launch failed: %w. Install a terminal emulator (foot, kitty, etc.)", err)}
		}
		cmd.Process.Release()
		return messages.StatusUpdateMsg{Status: model.StatusStopped}
	case "darwin":
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Terminal" to do script "%s"`, script))
		if err := cmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to launch in Terminal: %w", err)}
		}
		cmd.Process.Release()
		return messages.StatusUpdateMsg{Status: model.StatusStopped}
	default:
		return messages.ErrorMsg{Err: fmt.Errorf("unsupported OS: %s", runtime.GOOS)}
	}
}

// launchClaudeCodeWithConfig launches Claude Code connected to a local llama server
func (a *App) launchClaudeCodeWithConfig(m model.Model, port, ctxSize, workDir string) tea.Cmd {
	return func() tea.Msg {
		// 1. Start llama server with specific flags for Claude compatibility
		serverCmd := exec.Command("llama", "serve",
			"-hf", m.Config["model_name"],
			"--port", port,
			"--ctx-size", ctxSize,
			"--flash-attn", "on",
		)

		if err := serverCmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start llama server: %w", err)}
		}

		// 2. Wait for server to be ready
		time.Sleep(2 * time.Second)

		// 3. Setup Claude settings.json for KV cache performance
		{
			settingsDir := filepath.Join(os.Getenv("HOME"), ".claude")
			settingsPath := filepath.Join(settingsDir, "settings.json")
			if err := os.MkdirAll(settingsDir, 0o755); err == nil {
				var settings map[string]any
				data, err := os.ReadFile(settingsPath)
				if err == nil {
					json.Unmarshal(data, &settings)
				} else {
					settings = make(map[string]any)
				}
				if env, ok := settings["env"].(map[string]any); ok {
					env["CLAUDE_CODE_ATTRIBUTION_HEADER"] = "0"
				} else {
					settings["env"] = map[string]any{"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"}
				}
				if data, err := json.MarshalIndent(settings, "", "  "); err == nil {
					os.WriteFile(settingsPath, data, 0o644)
				}
			}
		}

		// 4. Launch Claude in terminal
		_ = serverCmd.Process.Release()
		claudeScript := fmt.Sprintf(`cd "%s" && ANTHROPIC_BASE_URL=http://localhost:%s claude --model my-model && read`, workDir, port)
		return spawnInTerminal(claudeScript)
	}
}

// launchOpencodeWithConfig launches Opencode connected to a local llama server
func (a *App) launchOpencodeWithConfig(m model.Model, port, ctxSize, workDir string) tea.Cmd {
	return func() tea.Msg {
		serverCmd := exec.Command("llama", "serve",
			"-hf", m.Config["model_name"],
			"--port", port,
			"--ctx-size", ctxSize,
			"--flash-attn", "on",
		)

		if err := serverCmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start llama server: %w", err)}
		}

		time.Sleep(2 * time.Second)

		configDir := filepath.Join(os.Getenv("HOME"), ".config", "opencode")
		configPath := filepath.Join(configDir, "opencode.json")

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to create config directory: %w", err)}
		}

		var config map[string]any
		data, err := os.ReadFile(configPath)
		if err != nil {
			config = map[string]any{
				"$schema": "https://opencode.ai/config.json",
			}
		} else {
			config = make(map[string]any)
			if err := json.Unmarshal(data, &config); err != nil {
				serverCmd.Process.Kill()
				return messages.ErrorMsg{Err: fmt.Errorf("failed to parse existing opencode config: %w", err)}
			}
			if config == nil {
				config = make(map[string]any)
			}
		}

		// Merge llama-cpp provider into existing config
		provider, _ := config["provider"].(map[string]any)
		if provider == nil {
			provider = make(map[string]any)
			config["provider"] = provider
		}
		provider["llama-cpp"] = map[string]any{
			"npm":  "@ai-sdk/openai-compatible",
			"name": "llama-cpp (local)",
			"options": map[string]any{
				"baseURL": fmt.Sprintf("http://localhost:%s/v1", port),
			},
			"models": map[string]any{
				m.Name: map[string]any{"name": m.Name},
			},
		}

		configData, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to marshal config: %w", err)}
		}

		if err := os.WriteFile(configPath, configData, 0o644); err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to write config: %w", err)}
		}

		_ = serverCmd.Process.Release()
		opencodeScript := fmt.Sprintf(`cd "%s" && opencode && read`, workDir)
		return spawnInTerminal(opencodeScript)
	}
}

// launchCrushWithConfig launches Crush connected to a local llama server
func (a *App) launchCrushWithConfig(m model.Model, port, ctxSize, workDir string) tea.Cmd {
	return func() tea.Msg {
		serverCmd := exec.Command("llama", "serve",
			"-hf", m.Config["model_name"],
			"--port", port,
			"--ctx-size", ctxSize,
			"--flash-attn", "on",
		)

		if err := serverCmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start llama server: %w", err)}
		}

		time.Sleep(2 * time.Second)

		configDir := filepath.Join(os.Getenv("HOME"), ".config", "crush")
		configPath := filepath.Join(configDir, "crush.json")

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to create config directory: %w", err)}
		}

		var config map[string]any
		data, err := os.ReadFile(configPath)
		if err != nil {
			config = map[string]any{
				"$schema": "https://charm.land/crush.json",
			}
		} else {
			config = make(map[string]any)
			if err := json.Unmarshal(data, &config); err != nil {
				serverCmd.Process.Kill()
				return messages.ErrorMsg{Err: fmt.Errorf("failed to parse existing crush config: %w", err)}
			}
			if config == nil {
				config = make(map[string]any)
			}
		}

		// Merge llama-cpp provider into existing config
		providers, _ := config["providers"].(map[string]any)
		if providers == nil {
			providers = make(map[string]any)
			config["providers"] = providers
		}
		ctxWindowSize, _ := strconv.ParseInt(ctxSize, 10, 64)
		providers["llama-cpp"] = map[string]any{
			"name":     "llama-cpp",
			"base_url": fmt.Sprintf("http://localhost:%s/v1/", port),
			"type":     "openai",
			"models": []map[string]any{
				{
					"name":           m.Name,
					"id":             fmt.Sprintf("%s-ctx-%s", m.Name, ctxSize),
					"context_window": ctxWindowSize,
				},
			},
		}

		configData, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to marshal config: %w", err)}
		}

		if err := os.WriteFile(configPath, configData, 0o644); err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to write config: %w", err)}
		}

		_ = serverCmd.Process.Release()
		crushScript := fmt.Sprintf(`cd "%s" && crush && read`, workDir)
		return spawnInTerminal(crushScript)
	}
}

// runBenchmark executes a benchmark for the given model
func (a *App) runBenchmark(m model.Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
		defer cancel()

		result, err := server.RunBenchmark(ctx, m)
		return messages.BenchmarkCompleteMsg{Result: result, Error: err}
	}
}

// renderBenchmarkView renders the benchmark results table
func (a *App) renderBenchmarkView() string {
	s := a.theme.Header.Render("Benchmark Results") + "\n\n"
	s += renderStatusPanes(a) + "\n"

	if a.benchmarking {
		s += fmt.Sprintf("%s Running benchmark for %s...\n\n", a.spin.View(), a.benchmarkModelName)
		s += "This may take a while.\n"
		hints := []string{"b Back"}
		s += a.theme.StatusBar("Benchmarking", hints, a.terminalWidth())
		return s
	}

	if len(a.benchmarkResults) == 0 {
		s += "No benchmarks run yet.\n\n"
		s += "Select a model then press 'm' to run a benchmark.\n"
		hints := []string{"b Back", "c Clear"}
		s += a.theme.StatusBar("Benchmark", hints, a.terminalWidth())
		return s
	}

	s += fmt.Sprintf("  %-4s %-50s %10s %10s %12s\n",
		"#", "Model", "PP t/s", "SGT t/s", "Date")
	s += a.renderBenchmarkDivider()

	for i, r := range a.benchmarkResults {
		name := r.ModelName
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		s += fmt.Sprintf("  %-4d %-50s %10.1f %10.1f %12s\n",
			i+1, name, r.PPTokensPerSecond,
			r.SGTTokensPerSecond, r.Timestamp[:10])
		if i < len(a.benchmarkResults)-1 {
			s += a.renderBenchmarkDivider()
		}
	}

	hints := []string{"b Back", "c Clear", "m Run Again"}
	s += a.theme.StatusBar("Benchmarks", hints, a.terminalWidth())
	return s
}

// renderBenchmarkDivider renders a rainbow-colored separator line between benchmark rows.
func (a *App) renderBenchmarkDivider() string {
	// Match format: "  %-4s %-50s %10s %10s %12s\n"
	divider := fmt.Sprintf("  %-4s %-50s %10s %10s %12s\n", "───", "──────────────────────────────────────────────", "─────────", "─────────", "───────────")
	colors := []lipgloss.Color{
		theme.ColorRed, theme.ColorOrange, theme.ColorYellow, theme.ColorLime,
		theme.ColorMint, theme.ColorCyan, theme.ColorLightBlue, theme.ColorPurple, theme.ColorPink,
	}
	runes := []rune(divider)
	chunkSize := len(runes) / len(colors)
	if chunkSize == 0 {
		chunkSize = 1
	}
	var result strings.Builder
	colorIdx := 0
	for i := 0; i < len(runes); i++ {
		if i > 0 && i%chunkSize == 0 && colorIdx < len(colors)-1 {
			colorIdx++
		}
		result.WriteString(lipgloss.NewStyle().Foreground(colors[colorIdx]).Render(string(runes[i])))
	}
	return result.String()
}

// renderCatalogView renders the model download catalog.
func (a *App) renderCatalogView() string {
	var s strings.Builder
	s.WriteString(a.theme.Header.Render("Model Catalog"))
	s.WriteString("\n\n")
	s.WriteString(renderStatusPanes(a))

	if a.downloadBusy {
		s.WriteString("\n  " + a.theme.Warning.Render(fmt.Sprintf("%s Downloading: %s", a.spin.View(), a.downloading)))
		hints := []string{"b Cancel", "esc Background"}
		s.WriteString(a.theme.StatusBar("Downloading", hints, a.terminalWidth()))
		return s.String()
	}

	if a.downloadProgress != "" {
		s.WriteString("\n  " + a.theme.Success.Render("✓ "+a.downloadProgress) + "\n")
	}

	if len(a.catalog) == 0 {
		s.WriteString(a.theme.Faint.Render("  No models in catalog.\n"))
		hints := []string{"b Back"}
		s.WriteString(a.theme.StatusBar("Catalog", hints, a.terminalWidth()))
		return s.String()
	}

	catalogLines := ""
	for i, m := range a.catalog {
		downloaded := server.IsModelDownloaded(m.HFRepo)
		status := a.theme.Faint.Render("○")
		if downloaded {
			status = a.theme.Success.Render("●")
		}
		if i == a.catalogCursor {
			prefix := a.theme.Success.Render("▸")
			catalogLines += fmt.Sprintf("  %s %s %s (%.1f GB)\n", prefix, status, m.DisplayName, m.SizeGB)
			if !downloaded {
				tags := strings.Join(m.Tags, ", ")
				catalogLines += fmt.Sprintf("      Tags: %s\n", tags)
			}
		} else {
			prefix := a.theme.Faint.Render(" ")
			catalogLines += fmt.Sprintf("  %s %s %s (%.1f GB)\n", prefix, status, m.DisplayName, m.SizeGB)
		}
	}

	s.WriteString(a.theme.PanelOrange.Render(a.theme.PanelTitle.Render("Catalog") + "\n" + catalogLines))
	hints := []string{"↑↓ Navigate", "Enter Download", "b Back"}
	s.WriteString(a.theme.StatusBar("Catalog", hints, a.terminalWidth()))
	return s.String()
}

// renderLaunchConfigView renders the launch configuration UI (port, context size, working dir).
func (a *App) renderLaunchConfigView() string {
	var s strings.Builder

	var appName string
	switch a.pendingLaunch {
	case model.LaunchClaudeCode:
		appName = "Claude Code"
	case model.LaunchOpencode:
		appName = "Opencode"
	case model.LaunchCrush:
		appName = "Crush"
	default:
		appName = "App"
	}

	s.WriteString(a.theme.Header.Render(fmt.Sprintf("Launch %s Config", appName)))
	s.WriteString("\n\n")
	s.WriteString(fmt.Sprintf("Model: %s\n\n", a.pendingModel.Name))
	s.WriteString(renderStatusPanes(a))

	s.WriteString(a.theme.Secondary.Render("Port: ") + a.portInput.View() + "\n")
	s.WriteString(a.theme.Secondary.Render("Context Size: ") + a.ctxSizeInput.View() + "\n")
	s.WriteString(a.theme.Secondary.Render("Working Dir: ") + a.workDirInput.View())

	if a.ctxSizeWarning != "" {
		s.WriteString("\n\n")
		s.WriteString(a.theme.Warning.Render(a.ctxSizeWarning) + "\n")
	}

	s.WriteString("\n")
	hints := []string{"Tab Switch", "Enter Launch", "b Back"}
	s.WriteString(a.theme.StatusBar(fmt.Sprintf("Launch %s", appName), hints, a.terminalWidth()))
	return s.String()
}

// renderKillServerView renders the kill server selection modal.
func (a *App) renderKillServerView() string {
	var s strings.Builder
	s.WriteString(a.theme.Header.Render("Kill Server"))
	s.WriteString("\n\n")
	s.WriteString("Select a server to kill, then press " + a.theme.Key.Render("k") + ".\n\n")

	for i, p := range a.runtimeServers {
		prefix := "    "
		if i == a.cursorPos {
			prefix = a.theme.Warning.Render("  ➤ ")
		}
		line := fmt.Sprintf("%d. PID %d", i+1, p.PID)
		if p.Port > 0 {
			line += fmt.Sprintf(" :%d", p.Port)
		}
		s.WriteString(prefix + line + "\n")
	}

	s.WriteString("\n")
	hints := []string{"↑↓ Navigate", "k Kill", "b Back"}
	s.WriteString(a.theme.StatusBar("Kill Server", hints, a.terminalWidth()))
	return s.String()
}

// renderSearchView renders the HuggingFace model search view.
func (a *App) renderSearchView() string {
	var s strings.Builder
	s.WriteString(a.theme.Header.Render("Search HuggingFace"))
	s.WriteString("\n\n")
	s.WriteString(renderStatusPanes(a))
	s.WriteString(a.searchInput.View())
	s.WriteString("\n\n")

	if a.searchBusy {
		s.WriteString(a.theme.Warning.Render(fmt.Sprintf("%s Searching for %s...", a.spin.View(), a.searchQuery)))
		hints := []string{"b Cancel"}
		s.WriteString(a.theme.StatusBar("Searching", hints, a.terminalWidth()))
		return s.String()
	}

	if a.downloadBusy {
		s.WriteString("\n  " + a.theme.Warning.Render(fmt.Sprintf("%s Downloading: %s", a.spin.View(), a.downloading)))
		hints := []string{"b Cancel", "esc Background"}
		s.WriteString(a.theme.StatusBar("Downloading", hints, a.terminalWidth()))
		return s.String()
	}

	if len(a.searchResults) == 0 && a.searchQuery != "" {
		s.WriteString(a.theme.Faint.Render("  No results found.\n"))
		hints := []string{"↑↓ Navigate", "Enter Quant", "b Back"}
		s.WriteString(a.theme.StatusBar("Search", hints, a.terminalWidth()))
		return s.String()
	}

	if len(a.searchResults) == 0 {
		s.WriteString(a.theme.Faint.Render("  Type a query and press Enter to search.\n"))
		hints := []string{"b Back"}
		s.WriteString(a.theme.StatusBar("Search", hints, a.terminalWidth()))
		return s.String()
	}

	resultLines := ""
	for i, r := range a.searchResults {
		prefix := a.theme.Faint.Render(" ")
		if i == a.searchCursor {
			prefix = a.theme.Success.Render("▸")
		}
		resultLines += fmt.Sprintf("  %s %s (%d downloads, %d likes)\n", prefix, r.ModelID, r.Downloads, r.Likes)
	}

	s.WriteString(a.theme.PanelOrange.Render(a.theme.PanelTitle.Render("Results") + "\n" + resultLines))
	hints := []string{"↑↓ Navigate", "Enter Quant", "b Back"}
	s.WriteString(a.theme.StatusBar("Search Results", hints, a.terminalWidth()))
	return s.String()
}

// renderQuantsView renders the quant file selection view.
func (a *App) renderQuantsView() string {
	var s strings.Builder
	s.WriteString(a.theme.Header.Render("Select Quantization"))
	s.WriteString("\n\n")
	s.WriteString(a.theme.Primary.Render(fmt.Sprintf("Repo: %s\n\n", a.selectedRepo)))
	s.WriteString(renderStatusPanes(a))

	if a.searchBusy {
		s.WriteString(a.theme.Warning.Render(fmt.Sprintf("%s Fetching files...", a.spin.View())))
		hints := []string{"b Cancel"}
		s.WriteString(a.theme.StatusBar("Fetching", hints, a.terminalWidth()))
		return s.String()
	}

	if len(a.repoFiles) == 0 {
		s.WriteString(a.theme.Faint.Render("  No GGUF files found in this repo.\n"))
		hints := []string{"b Back"}
		s.WriteString(a.theme.StatusBar("Quants", hints, a.terminalWidth()))
		return s.String()
	}

	fileLines := ""
	const maxLines = 15
	total := len(a.repoFiles)
	// Clamp cursor to valid range
	if a.fileCursor < 0 {
		a.fileCursor = 0
	}
	if a.fileCursor >= total {
		a.fileCursor = max(0, total-1)
	}
	if total > maxLines {
		half := maxLines / 2
		if a.fileCursor < half {
			a.fileScrollOffset = 0
		} else if a.fileCursor >= total-half {
			a.fileScrollOffset = total - maxLines
		} else {
			a.fileScrollOffset = a.fileCursor - half
		}
	} else {
		a.fileScrollOffset = 0
	}
	a.fileScrollOffset = max(0, a.fileScrollOffset)
	if total > maxLines {
		a.fileScrollOffset = min(a.fileScrollOffset, total-maxLines)
	}
	for i := a.fileScrollOffset; i < a.fileScrollOffset+maxLines && i < total; i++ {
		f := a.repoFiles[i]
		prefix := a.theme.Faint.Render(" ")
		if i == a.fileCursor {
			prefix = a.theme.Success.Render("▸")
		}
		fileLines += fmt.Sprintf("  %s %s (%s)\n", prefix, f.Filename, server.FormatSize(f.Size))
	}

	s.WriteString(a.theme.PanelBlue.Render(a.theme.PanelTitle.Render("Available Files") + "\n" + fileLines))
	hints := []string{"↑↓ Navigate", "Enter Add", "b Back"}
	if len(a.repoFiles) > maxLines {
		s.WriteString(fmt.Sprintf("Showing %d-%d of %d",
			a.fileScrollOffset+1, min(a.fileScrollOffset+maxLines, len(a.repoFiles)), len(a.repoFiles)))
		s.WriteString(a.theme.StatusBar("Quants", hints, a.terminalWidth()))
	} else {
		s.WriteString(a.theme.StatusBar("Quants", hints, a.terminalWidth()))
	}
	return s.String()
}
