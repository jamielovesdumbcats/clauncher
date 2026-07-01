package messages

import (
	"time"

	"clauncher/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
)

// ModelSelectedMsg is sent when a model is chosen from the selection view.
type ModelSelectedMsg struct {
	Selected model.Model
}

// LogMsg is sent when a new log line is received from a running process.
type LogMsg struct {
	Line string
}

// StatusUpdateMsg is sent when the process status changes (e.g., running, crashed).
type StatusUpdateMsg struct {
	Status model.ProcessStatus
	Error  error
}

// StatusTickMsg is sent periodically to update the status display.
type StatusTickMsg struct{}

// ErrorMsg is sent for fatal errors in the application.
type ErrorMsg struct {
	Err error
}

// StatusTickCmd creates a command that returns a StatusTickMsg after a short delay.
func StatusTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return StatusTickMsg{}
	}
}

// ModelsRefreshedMsg is sent when the local model list has been refreshed.
type ModelsRefreshedMsg struct {
	Models []model.Model
	Error  error
}
