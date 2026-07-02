package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
	"clauncher/pkg/ui/messages"
	"clauncher/pkg/ui/theme"

	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	ViewSelection ViewState = iota
	ViewDashboard
	ViewLaunchOptions
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

	// Context for process management
	ctx      context.Context
	cancelFn context.CancelFunc
}

// NewApp initializes a new application instance.
func NewApp(models []model.Model, runner server.ProcessRunner) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		currentView: ViewSelection,
		theme:       theme.NewTheme(),
		models:      models,
		runner:      runner,
		logs:        []string{},
		ctx:         ctx,
		cancelFn:    cancel,
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
			// Go back to selection from dashboard
			if a.currentView == ViewDashboard {
				return a, a.goBack()
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
				return a, func() tea.Msg {
					return messages.LaunchOptionSelectedMsg{
						Option: option,
						Model:  *a.pendingModel,
					}
				}
			}
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
			// Launch Claude Code with local model
			return a, a.launchClaudeCode(m.Model)
		case model.LaunchOpencode:
			// Launch Opencode with local model
			return a, a.launchOpencode(m.Model)
		case model.LaunchCrush:
			// Launch Crush with local model
			return a, a.launchCrush(m.Model)
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
	if a.currentView == ViewLaunchOptions {
		a.currentView = ViewSelection
		a.pendingModel = nil
	} else if a.currentView == ViewDashboard {
		a.currentView = ViewSelection
		a.selectedModel = nil
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
	default:
		return "Unknown View"
	}
}

// renderSelectionView renders the model selection UI.
func (a *App) renderSelectionView() string {
	s := a.theme.Header.Render("Clauncher - Select a Model") + "\n\n"

	// Show loading indicator if refreshing
	if a.refreshing {
		s += "Refreshing model list...\n\n"
	}

	// Show error if refresh failed
	if a.err != nil && a.refreshing {
		s += a.theme.Error.Render(fmt.Sprintf("Error refreshing models: %v\n", a.err)) + "\n"
	}

	if len(a.models) == 0 {
		if !a.refreshing {
			s += "No models found. Press 'r' to refresh the model list.\n"
		}
	} else {
		for i, m := range a.models {
			s += fmt.Sprintf("  %d. %s\n", i+1, m.Name)
		}
	}

	s += "\nPress 1 to select the first model"
	if len(a.models) > 1 {
		s += ", 2-N for other models"
	}
	s += ", r to refresh list, q to quit"

	return s
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
	s += "\nPress 1-5 to select, b to go back"
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
		var cmd *exec.Cmd
		var err error

		switch runtime.GOOS {
		case "linux":
			// Try common terminal emulators on Linux
			terminals := []string{"gnome-terminal", "konsole", "xterm", "terminator"}
			for _, term := range terminals {
				if _, err := exec.LookPath(term); err == nil {
					cmd = exec.Command(term, "-e", "sh", "-c", fmt.Sprintf(
						"llama --model %s && %s -e read", m.Config["model_name"], term))
					err = cmd.Start()
					if err == nil {
						cmd.Process.Release()
						return messages.StatusUpdateMsg{Status: model.StatusStopped}
					}
				}
			}
			// Fallback to basic shell execution
			cmd = exec.Command("sh", "-c", fmt.Sprintf("llama --model %s", m.Config["model_name"]))
		case "darwin":
			// macOS - use osascript to open Terminal
			cmd = exec.Command("osascript", "-e",
				fmt.Sprintf(`tell application "Terminal" to do script "llama --model %s"`, m.Config["model_name"]))
		default:
			return messages.ErrorMsg{Err: fmt.Errorf("unsupported OS: %s", runtime.GOOS)}
		}

		if err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to launch CLI: %w", err)}
		}

		cmd.Process.Release()
		return messages.StatusUpdateMsg{Status: model.StatusStopped}
	}
}

// launchClaudeCode launches Claude Code connected to a local llama server
func (a *App) launchClaudeCode(m model.Model) tea.Cmd {
	return func() tea.Msg {
		port := "8081" // Default port

		// 1. Start llama server with specific flags for Claude compatibility
		serverCmd := exec.Command("llama", "serve",
			"-hf", m.Config["model_name"],
			"--port", port,
			"--ctx-size", "131072", // Large context
			"--flash-attn", "on",   // Performance
		)

		if err := serverCmd.Start(); err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start llama server: %w", err)}
		}

		// 2. Wait for server to be ready
		time.Sleep(2 * time.Second)

		// 3. Set environment variable and launch Claude
		envCmd := exec.Command("sh", "-c",
			fmt.Sprintf(`export ANTHROPIC_BASE_URL=https://localhost:%s && claude --model my-model`, port))

		if err := envCmd.Start(); err != nil {
			serverCmd.Process.Kill()
			return messages.ErrorMsg{Err: fmt.Errorf("failed to launch claude: %w", err)}
		}

		envCmd.Process.Release()

		// Return to selection view - processes run independently
		return messages.StatusUpdateMsg{Status: model.StatusStopped}
	}
}
