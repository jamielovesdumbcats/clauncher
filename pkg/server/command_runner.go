package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"clauncher/pkg/model"
)

// ProcessInfo holds metadata about a running or recently terminated process.
type ProcessInfo struct {
	Status  model.ProcessStatus
	Logs    []string
	Error   error
}

// ProcessRunner defines the interface for managing external command lifecycles.
type ProcessRunner interface {
	Start(ctx context.Context, m model.Model) (<-chan string, error)
	Stop() error
	Status() ProcessInfo
	ClearError() // Clear any stored error state
}

// CommandRunner implements ProcessRunner for real OS processes.
type CommandRunner struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	status         model.ProcessStatus
	logs           []string
	err            error
	logChan        chan string
	logChanClosed  bool
	stopFunc       context.CancelFunc
	procCtx        context.Context // Context for the running process
	commandBuilder func(m model.Model) (string, []string)
}

// NewCommandRunner creates a new runner with a specific command builder.
// The commandBuilder allows different implementations for Claude and Llama.
func NewCommandRunner(builder func(m model.Model) (string, []string)) *CommandRunner {
	return &CommandRunner{
		status:         model.StatusStopped,
		commandBuilder: builder,
		logChan:        make(chan string, 100),
		logChanClosed:  false,
	}
}

// Start launches the process defined by the command builder.
func (r *CommandRunner) Start(ctx context.Context, m model.Model) (<-chan string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == model.StatusRunning {
		return nil, fmt.Errorf("process is already running")
	}

	r.status = model.StatusStarting

	// Create a cancellable context for this specific process run
	runCtx, cancel := context.WithCancel(ctx)
	r.stopFunc = cancel
	r.procCtx = runCtx

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

	r.status = model.StatusRunning

	// Start log monitoring
	go r.monitorPipes(stdout, stderr)
	go r.monitorExit()

	return r.logChan, nil
}

// Stop attempts to gracefully stop the process.
func (r *CommandRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != model.StatusRunning && r.status != model.StatusStarting {
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
	// Wait for pipes to finish first to avoid race condition
	// We need to ensure monitorPipes goroutines have finished reading
	// before we close the channel

	err := r.cmd.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil {
		// Check if it was a normal exit or a crash
		if exitError, ok := err.(*exec.ExitError); ok {
			// Check if it was a signal-based exit (likely intentional stop)
			if exitError.ProcessState.Success() {
				r.status = model.StatusStopped
			} else {
				// Check if the exit was due to context cancellation (intentional stop)
				// vs an actual crash
				if r.procCtx.Err() != nil {
					// Context was cancelled - this is an intentional stop
					r.status = model.StatusStopped
				} else {
					r.status = model.StatusCrashed
					r.err = err
				}
			}
		} else if err == context.Canceled || err == context.DeadlineExceeded {
			// Context cancellation - intentional stop
			r.status = model.StatusStopped
		} else {
			r.status = model.StatusCrashed
			r.err = err
		}
	} else {
		r.status = model.StatusStopped
	}

	// Ensure the log channel is closed so receivers stop waiting
	// Only close if not already closed
	if !r.logChanClosed {
		close(r.logChan)
		r.logChanClosed = true
	}
}

func (r *CommandRunner) addLog(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, msg)

	// Non-blocking send to log channel, but check if closed first
	if !r.logChanClosed {
		select {
		case r.logChan <- msg:
		default:
			// Channel full, skip to prevent blocking
		}
	}
}

func (r *CommandRunner) cleanup(ctx context.Context) {
	r.status = model.StatusStopped
	if r.stopFunc != nil {
		r.stopFunc()
	}
}

// ClearError clears any stored error state. Called when intentionally stopping a process.
func (r *CommandRunner) ClearError() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = nil
}

// ListLocalModels runs "llama serve -cl" and parses the output to return a list of locally cached models.
func ListLocalModels() ([]model.Model, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "llama", "serve", "-cl")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run llama serve -cl: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var models []model.Model

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header lines and empty lines
		if line == "" || strings.Contains(line, "number of models") {
			continue
		}

		// Parse lines like "   1. mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS"
		// Match pattern: N. path:quant
		parts := strings.SplitN(line, ". ", 2)
		if len(parts) != 2 {
			continue
		}

		// Extract the model path (without the quant suffix after colon)
		modelPath := parts[1]
		if idx := strings.Index(modelPath, ":"); idx != -1 {
			modelPath = modelPath[:idx]
		}

		if modelPath == "" {
			continue
		}

		// Create a display name from the model path (last component)
		displayName := modelPath
		if idx := strings.LastIndex(modelPath, "/"); idx != -1 {
			// Keep org/repo format, preserve size info (e.g., 9B, 35B)
			namePart := modelPath[idx+1:]
			// Remove -GGUF and quant suffix for cleaner display
			// e.g., "Qwen3.6-27B-GGUF:IQ4_XS" -> "Qwen3.6-27B"
			if idx2 := strings.Index(namePart, "-GGUF"); idx2 != -1 {
				displayName = modelPath[:idx+1] + namePart[:idx2]
			} else {
				displayName = namePart
			}
		}

		models = append(models, model.Model{
			ID:   strings.ReplaceAll(modelPath, "/", "-"),
			Name: displayName,
			Type: model.LlamaCPP,
			Config: map[string]string{
				"model_name": parts[1], // Full path with quant
			},
		})
	}

	return models, nil
}

