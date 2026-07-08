package messages

import (
	"time"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
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

// LaunchOptionSelectedMsg is sent when user selects a launch option
type LaunchOptionSelectedMsg struct {
	Option model.LaunchOption
	Model  model.Model
}

// BenchmarkCompleteMsg is sent when a benchmark run finishes.
type BenchmarkCompleteMsg struct {
	Result *model.BenchmarkResult
	Error  error
}

// DownloadCompleteMsg is sent when a model download finishes.
type DownloadCompleteMsg struct {
	Model string
	Error error
}

// SearchCompleteMsg is sent when HuggingFace search results are returned.
type SearchCompleteMsg struct {
	Results []server.SearchHFModel
	Error   error
}

// FilesCompleteMsg is sent when repo files are fetched.
type FilesCompleteMsg struct {
	Files []server.HFFile
	Error error
}

// ModelAddedToCatalogMsg is sent when a model is added to the catalog.
type ModelAddedToCatalogMsg struct {
	Model server.CatalogModel
	Error error
}

// SuccessMsg is sent for success notifications.
type SuccessMsg struct {
	Message string
}
