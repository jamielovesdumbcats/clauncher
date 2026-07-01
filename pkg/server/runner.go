package server

import (
	"context"
	"fmt"
	"time"

	"clauncher/pkg/model"
)

type ProcessStatus string

const (
	StatusRunning  ProcessStatus = "running"
	StatusStopped  ProcessStatus = "stopped"
	StatusCrashed  ProcessStatus = "crashed"
	StatusStarting ProcessStatus = "starting"
)

type ProcessInfo struct {
	Status  ProcessStatus
	Logs    []string
	Error   error
}

type ProcessRunner interface {
	Start(ctx context.Context, m model.Model) (<-chan string, error)
	Stop() error
	Status() ProcessInfo
}

// MockRunner is used for testing purposes to simulate process behavior.
type MockRunner struct {
	currentStatus ProcessStatus
	logs          []string
	err           error
	stopChan      chan struct{}
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		currentStatus: StatusStopped,
		stopChan:      make(chan struct{}),
	}
}

func (r *MockRunner) Start(ctx context.Context, m model.Model) (<-chan string, error) {
	if r.currentStatus == StatusRunning {
		return nil, fmt.Errorf("process is already running")
	}

	r.currentStatus = StatusRunning
	logChan := make(chan string, 10)

	go func() {
		defer close(logChan)

		// Simulate startup log
		r.addLog("Starting mock process...")
		logChan <- "Starting mock process..."

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				r.currentStatus = StatusStopped
				r.addLog("Process stopped via context.")
				logChan <- "Process stopped via context."
				return
			case <-r.stopChan:
				r.currentStatus = StatusStopped
				r.addLog("Process stopped via stop signal.")
				logChan <- "Process stopped via stop signal."
				return
			case <-ticker.C:
				msg := fmt.Sprintf("Mock log at %s", time.Now().Format(time.Kitchen))
				r.addLog(msg)
				logChan <- msg
			}
		}
	}()

	return logChan, nil
}

func (r *MockRunner) Stop() error {
	if r.currentStatus != StatusRunning {
		return fmt.Errorf("process is not running")
	}
	close(r.stopChan)
	r.stopChan = make(chan struct{})
	return nil
}

func (r *MockRunner) Status() ProcessInfo {
	return ProcessInfo{
		Status: r.currentStatus,
		Logs:   r.logs,
		Error:  r.err,
	}
}

func (r *MockRunner) addLog(msg string) {
	r.logs = append(r.logs, msg)
}
