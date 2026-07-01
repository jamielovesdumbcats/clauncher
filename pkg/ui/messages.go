package messages

import (
	"clauncher/pkg/model"
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

// ErrorMsg is sent for fatal errors in the application.
type ErrorMsg struct {
	Err error
}
