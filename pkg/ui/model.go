package ui

import (
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
}

// NewApp initializes a new application instance.
func NewApp(models []model.Model, runner server.ProcessRunner) *App {
	return &App{
		currentView: ViewSelection,
		theme:       theme.NewTheme(),
		models:      models,
		runner:      runner,
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
		}
	case messages.ModelSelectedMsg:
		a.selectedModel = &m.Selected
		a.currentView = ViewDashboard
		return a, a.startProcess(m.Selected)

	case messages.LogMsg:
		// In a real app, we would update the dashboard state here
		return a, nil

	case messages.StatusUpdateMsg:
		// Update dashboard status
		return a, nil

	case messages.ErrorMsg:
		a.err = m.Err
		return a, nil
	}

	return a, nil
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
	s := a.theme.Header.Render(fmt.Sprintf("Dashboard: %s", a.selectedModel.Name)) + "\n"
	s += a.theme.Border.Render("Process is running...")
	return s
}

// selectModel handles the transition from selection to dashboard
func (a *App) selectModel(idx int) tea.Cmd {
	return func() tea.Msg {
		return messages.ModelSelectedMsg{Selected: a.models[idx]}
	}
}

// startProcess is a helper to trigger process start via the runner.
func (a *App) startProcess(m model.Model) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder for real async execution
		// In Phase 4, we will integrate the real runner here
		return messages.ModelSelectedMsg{Selected: m}
	}
}
