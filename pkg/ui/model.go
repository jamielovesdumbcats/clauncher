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

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	ViewSelection ViewState = iota
	ViewDashboard
	ViewLaunchOptions
	ViewBenchmark
	ViewLaunchConfig
)

// App is the root model for the application.
type App struct {
	currentView ViewState
	theme       *theme.Theme

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
}

// NewApp initializes a new application instance.
func NewApp(models []model.Model, runner server.ProcessRunner) *App {
	ctx, cancel := context.WithCancel(context.Background())
	store := model.NewBenchmarkStore("")
	results, _ := store.Load()

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
	}
}

// Init starts the application.
func (a *App) Init() tea.Cmd {
	return nil
}

// Update handles all incoming messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
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
		case "b", "esc":
			// Go back to selection from dashboard or benchmark view
			if a.currentView == ViewDashboard || a.currentView == ViewLaunchOptions || a.currentView == ViewBenchmark || a.currentView == ViewLaunchConfig {
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
				return a, a.runBenchmark(*a.pendingModel)
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
			return a, a.launchLlamaCLI(m.Model)
		case model.LaunchClaudeCode:
			// Launch Claude Code with local model using config
			return a, a.launchClaudeCodeWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value())
		case model.LaunchOpencode:
			// Launch Opencode with local model using config
			return a, a.launchOpencodeWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value())
		case model.LaunchCrush:
			// Launch Crush with local model using config
			return a, a.launchCrushWithConfig(m.Model, a.portInput.Value(), a.ctxSizeInput.Value(), a.workDirInput.Value())
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
	} else if a.currentView == ViewBenchmark {
		a.currentView = ViewSelection
	}
	a.err = nil // Clear any error state when returning to selection
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

// View renders the current application state.
func (a *App) View() string {
	if a.err != nil {
		return a.theme.Error.Render(fmt.Sprintf("Error: %v\n\nPress 'b' to go back", a.err))
	}

	switch a.currentView {
	case ViewSelection:
		return a.renderSelectionView()
	case ViewLaunchOptions:
		return a.renderLaunchOptionsView()
	case ViewDashboard:
		return a.renderDashboardView()
	case ViewBenchmark:
		return a.renderBenchmarkView()
	case ViewLaunchConfig:
		return a.renderLaunchConfigView()
	default:
		return "Unknown View"
	}
}

// renderSelectionView renders the model selection UI.
func (a *App) renderSelectionView() string {
	var s strings.Builder
	s.WriteString(a.theme.Header.Render("Clauncher - Select a Model"))
	s.WriteString("\n\n")

	// Show loading indicator if refreshing
	if a.refreshing {
		s.WriteString("Refreshing model list...\n\n")
	}

	// Show error if refresh failed
	if a.err != nil && a.refreshing {
		s.WriteString(a.theme.Error.Render(fmt.Sprintf("Error refreshing models: %v", a.err)))
		s.WriteString("\n\n")
	}

	if len(a.models) == 0 {
		if !a.refreshing {
			s.WriteString("No models found. Press 'r' to refresh the model list.\n")
		}
	} else {
		for i, m := range a.models {
			fmt.Fprintf(&s, "  %d. %s\n", i+1, m.Name)
		}
	}

	s.WriteString("\nPress 1 to select the first model")
	if len(a.models) > 1 {
		s.WriteString(", 2-N for other models")
	}
	s.WriteString(", r to refresh, m for benchmarks, q to quit")

	return s.String()
}

// renderLaunchOptionsView renders the launch options UI.
func (a *App) renderLaunchOptionsView() string {
	s := a.theme.Header.Render("Launch Options") + "\n\n"
	s += a.theme.Primary.Render(fmt.Sprintf("Model: %s\n\n", a.pendingModel.Name))
	s += "  1. " + a.theme.Success.Render("Launch Llama Server") + " (open in browser)\n"
	s += "  2. " + a.theme.Secondary.Render("Launch Llama CLI") + " (new terminal)\n"
	s += "  3. " + a.theme.Info.Render("Launch Claude Code") + " (with local model)\n"
	s += "  4. " + a.theme.Info.Render("Launch Opencode") + " (with local model)\n"
	s += "  5. " + a.theme.Info.Render("Launch Crush") + " (with local model)\n"
	s += "  6. " + a.theme.Info.Render("Run Benchmark") + " (for this model)\n"
	s += "\nPress 1-6 to select, b to go back"
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
	logBox := a.theme.Border.Render(logContent)

	// Control hints
	controlHint := "[s] toggle start/stop | [b] back | [q] quit"
	if status == model.StatusRunning {
		controlHint = "[s] stop | [b] back | [q] quit"
	}

	footer := fmt.Sprintf("\n%s", controlHint)

	return fmt.Sprintf("%s\n\n%s\n\n%s", styledHeader, logBox, a.theme.Footer.Render(footer))
}

// selectModel handles the transition from selection to launch options
func (a *App) selectModel(idx int) tea.Cmd {
	a.pendingModel = &a.models[idx]
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

	if a.benchmarking {
		s += fmt.Sprintf("Running benchmark for %s...\n\n", a.benchmarkModelName)
		s += "This may take a while.\n"
		s += "\nPress 'b' to go back"
		return s
	}

	if len(a.benchmarkResults) == 0 {
		s += "No benchmarks run yet.\n\n"
		s += "Select a model then press 'm' to run a benchmark.\n"
		s += "\nb: go back | c: clear"
		return s
	}

	s += fmt.Sprintf("%-4s %-30s %10s %10s %10s\n",
		"#", "Model", "PP t/s", "SGT t/s", "Date")
	s += "─────────────────────────────────────────────────────────────────────\n"

	for i, r := range a.benchmarkResults {
		name := r.ModelName
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		s += fmt.Sprintf("%-4d %-30s %10.1f %10.1f %10s\n",
			i+1, name, r.PPTokensPerSecond,
			r.SGTTokensPerSecond, r.Timestamp[:10])
	}

	s += "\nb: go back | c: clear"
	return s
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

	s.WriteString(a.theme.Secondary.Render("Port: ") + a.portInput.View() + "\n")
	s.WriteString(a.theme.Secondary.Render("Context Size: ") + a.ctxSizeInput.View() + "\n")
	s.WriteString(a.theme.Secondary.Render("Working Dir: ") + a.workDirInput.View())

	if a.ctxSizeWarning != "" {
		s.WriteString("\n\n")
		s.WriteString(a.theme.Warning.Render(a.ctxSizeWarning) + "\n")
	}

	s.WriteString("\n")
	s.WriteString("Tab: switch field | Enter: launch | b: go back")

	return s.String()
}
