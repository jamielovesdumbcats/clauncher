package ui

import (
	"context"
	"fmt"

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
	logs     []string
	logChan  <-chan string
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
			return a, tea.Quit
		case "1":
			if len(a.models) > 0 {
				return a, a.selectModel(0)
			}
		case "s":
			if a.currentView == ViewDashboard {
				return a, a.toggleProcess()
			}
		}
	case messages.ModelSelectedMsg:
		a.selectedModel = &m.Selected
		a.currentView = ViewDashboard
		return a, a.startProcess(m.Selected)

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
		return a, nil

	case messages.ErrorMsg:
		a.err = m.Err
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
		return func() tea.Msg {
			err := a.runner.Stop()
			if err != nil {
				return messages.ErrorMsg{Err: err}
			}
			return messages.StatusUpdateMsg{Status: model.StatusStopped}
		}
	} else {
		return a.startProcess(*a.selectedModel)
	}
}

// View renders the current application state.
func (a *App) View() string {
	if a.err != nil {
		return a.theme.Error.Render(fmt.Sprintf("Error: %v", a.err))
	}

	switch a.currentView {
	case ViewSelection:
		return a.renderSelectionView()
	case ViewDashboard:
		return a.renderDashboardView()
	default:
		return "Unknown View"
	}
}

// renderSelectionView renders the model selection UI.
func (a *App) renderSelectionView() string {
	s := a.theme.Header.Render("Select a Model to Launch") + "\n\n"
	for i, m := range a.models {
		s += fmt.Sprintf("%d. %s\n", i+1, m.Name)
	}
	s += "\n(Press 1 to select, q to quit)"
	return s
}

// renderDashboardView renders the main dashboard UI.
func (a *App) renderDashboardView() string {
	header := a.theme.Header.Render(fmt.Sprintf("Dashboard: %s", a.selectedModel.Name))

	// Construct log area
	logContent := ""
	if len(a.logs) == 0 {
		logContent = "Waiting for logs..."
	} else {
		for _, line := range a.logs {
			logContent += line + "\n"
		}
	}

	// Use a scrollable-like view for logs
	logBox := a.theme.Border.Render(logContent)

	status := a.runner.Status().Status
	controlHint := "[s] toggle start/stop"
	if status == model.StatusRunning {
		controlHint = "[s] stop"
	}

	footer := fmt.Sprintf("\nStatus: %s | %s", status, controlHint)

	return fmt.Sprintf("%s\n\n%s\n\n%s", header, logBox, a.theme.Footer.Render(footer))
}

// selectModel handles the transition from selection to dashboard
func (a *App) selectModel(idx int) tea.Cmd {
	return func() tea.Msg {
		return messages.ModelSelectedMsg{Selected: a.models[idx]}
	}
}

// startProcess starts the process and returns a command that reads logs.
func (a *App) startProcess(m model.Model) tea.Cmd {
	return func() tea.Msg {
		// Start the process
		logChan, err := a.runner.Start(a.ctx, m)
		if err != nil {
			return messages.ErrorMsg{Err: fmt.Errorf("failed to start process: %w", err)}
		}

		// Store the log channel
		a.logChan = logChan

		// Start reading logs in a goroutine
		go func() {
			for line := range logChan {
				// Send log message to UI via a command
				// We use a timer-based approach to batch messages
				a.logs = append(a.logs, line)
				if len(a.logs) > 100 {
					a.logs = a.logs[1:]
				}
			}
			// Channel closed - process exited
			status := a.runner.Status()
			if status.Error != nil {
				// Process crashed
			}
		}()

		return messages.StatusUpdateMsg{Status: model.StatusRunning}
	}
}
