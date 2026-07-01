package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"clauncher/pkg/model"
)

// ProcessStatus represents the current state of a running process.
type ProcessStatus string

const (
	StatusRunning  ProcessStatus = "running"
	StatusStopped  ProcessStatus = "stopped"
	StatusCrashed  ProcessStatus = "crashed"
	StatusStarting ProcessStatus = "starting"
)

// ProcessInfo holds metadata about a running or recently terminated process.
type ProcessInfo struct {
	Status  ProcessStatus
	Logs    []string
	Error   error
}

// ProcessRunner defines the interface for managing external command lifecycles.
type ProcessRunner interface {
	Start(ctx context.Context, m model.Model) (<-chan string, error)
	Stop() error
	Status() ProcessInfo
}

// CommandRunner implements ProcessRunner for real OS processes.
type CommandRunner struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	status         ProcessStatus
	logs           []string
	err            error
	logChan        chan string
	stopFunc       context.CancelFunc
	commandBuilder func(m model.Model) (string, []string)
}

// NewCommandRunner creates a new runner with a specific command builder.
// The commandBuilder allows different implementations for Claude and Llama.
func NewCommandRunner(builder func(m model.Model) (string, []string)) *CommandRunner {
	return &CommandRunner{
		status:         StatusStopped,
		commandBuilder: builder,
		logChan:        make(chan string, 100),
	}
}

// Start launches the process defined by the command builder.
func (r *CommandRunner) Start(ctx context.Context, m model.Model) (<-chan string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == StatusRunning {
		return nil, fmt.Errorf("process is already running")
	}

	r.status = StatusStarting

	// Create a cancellable context for this specific process run
	runCtx, cancel := context.WithCancel(ctx)
	r.stopFunc = cancel

	name, args := r.commandBuilder(m)
	r.cmd = exec.CommandContext(runCtx, name, args...)

	// Set up pipes for stdout and stderr
	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		r.cleanup(runCtx)
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		r.cleanup(runCtx)
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := r.cmd.Start(); err != nil {
		r.cleanup(runCtx)
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	r.status = StatusRunning

	// Start log monitoring
	go r.monitorPipes(stdout, stderr)
	go r.monitorExit()

	return r.logChan, nil
}

// Stop attempts to gracefully stop the process.
func (r *CommandRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != StatusRunning && r.status != StatusStarting {
		return fmt.Errorf("process is not running")
	}

	if r.stopFunc != nil {
		r.stopFunc()
	}
	return nil
}

// Status returns the current state of the process.
func (r *CommandRunner) Status() ProcessInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ProcessInfo{
		Status: r.status,
		Logs:   r.logs,
		Error:  r.err,
	}
}

func (r *CommandRunner) monitorPipes(stdout, stderr io.ReadCloser) {
	var wg sync.WaitGroup
	wg.Add(2)

	pipeFunc := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			r.addLog(line)
		}
	}

	go pipeFunc(stdout)
	go pipeFunc(stderr)

	wg.Wait()
}

func (r *CommandRunner) monitorExit() {
	err := r.cmd.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil {
		// Check if it was a normal exit or a crash
		if exitError, ok := err.(*exec.ExitError); ok {
			// Check if it was a signal-based exit (likely intentional stop)
			if exitError.ProcessState.Success() {
				r.status = StatusStopped
			} else {
				r.status = StatusCrashed
				r.err = err
			}
		} else {
			r.status = StatusCrashed
			r.err = err
		}
	} else {
		r.status = StatusStopped
	}

	// Ensure the log channel is closed so receivers stop waiting
	close(r.logChan)
}

func (r *CommandRunner) addLog(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, msg)

	// Non-blocking send to log channel
	select {
	case r.logChan <- msg:
	default:
		// Channel full, skip to prevent blocking
	}
}

func (r *CommandRunner) cleanup(ctx context.Context) {
	r.status = StatusStopped
	if r.stopFunc != nil {
		r.stopFunc()
	}
}
