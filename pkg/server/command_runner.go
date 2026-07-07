package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"clauncher/pkg/model"
)

// ProcessInfo holds metadata about a running or recently terminated process.
type ProcessInfo struct {
	Status model.ProcessStatus
	Logs   []string
	Error  error
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

func (r *CommandRunner) cleanup(_ context.Context) {
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

// RunningLlamaProcess holds info about a detected running llama server.
type RunningLlamaProcess struct {
	PID  int
	Port int
}

// FindRunningLlamaServers looks for running llama server processes using pgrep/ps.
// Returns a list of PIDs and their ports if detectable.
func FindRunningLlamaServers() ([]RunningLlamaProcess, error) {
	// Try pgrep first (Linux/macOS)
	if _, err := exec.LookPath("pgrep"); err == nil {
		cmd := exec.Command("pgrep", "-f", "llama.*serve")
		output, err := cmd.Output()
		if err != nil {
			return nil, nil // No processes found
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		var procs []RunningLlamaProcess
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pid, err := strconv.Atoi(line)
			if err != nil {
				continue
			}
			procs = append(procs, RunningLlamaProcess{PID: pid})
		}
		return procs, nil
	}

	// Fallback: try ps
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ps: %w", err)
	}

	var procs []RunningLlamaProcess
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "llama") && strings.Contains(line, "serve") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pid, err := strconv.Atoi(parts[1])
				if err == nil {
					procs = append(procs, RunningLlamaProcess{PID: pid})
				}
			}
		}
	}
	return procs, nil
}

// KillLlamaServer sends SIGTERM to a process by PID.
func KillLlamaServer(pid int) error {
	cmd := exec.Command("kill", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

// KillLlamaServers sends SIGTERM to all detected running llama servers.
func KillLlamaServers() error {
	procs, err := FindRunningLlamaServers()
	if err != nil {
		return err
	}
	for _, p := range procs {
		if err := KillLlamaServer(p.PID); err != nil {
			return fmt.Errorf("failed to kill PID %d: %w", p.PID, err)
		}
	}
	return nil
}

// MockRunner implements ProcessRunner for UI testing without launching real processes.
type MockRunner struct {
	status  model.ProcessStatus
	logs    []string
	err     error
	logChan chan string
}

// NewMockRunner creates a new MockRunner instance.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		status:  model.StatusStopped,
		logChan: make(chan string, 100),
	}
}

// Start simulates starting a process by sending mock logs.
func (r *MockRunner) Start(ctx context.Context, m model.Model) (<-chan string, error) {
	r.status = model.StatusRunning
	go func() {
		r.logChan <- "[mock] Starting " + m.Name
		time.Sleep(100 * time.Millisecond)
		r.logChan <- "[mock] " + m.Name + " is running"
		close(r.logChan)
	}()
	return r.logChan, nil
}

// Stop simulates stopping the process.
func (r *MockRunner) Stop() error {
	r.status = model.StatusStopped
	return nil
}

// Status returns the current mock status.
func (r *MockRunner) Status() ProcessInfo {
	return ProcessInfo{
		Status: r.status,
		Logs:   r.logs,
		Error:  r.err,
	}
}

// ClearError clears any stored error state.
func (r *MockRunner) ClearError() {
	r.err = nil
}

// CatalogModel represents a model available for download from HuggingFace.
type CatalogModel struct {
	HFRepo      string   `json:"hf_repo"`
	DisplayName string   `json:"display_name"`
	SizeGB      float64  `json:"size_gb"`
	Tags        []string `json:"tags"`
}

// Catalog holds a list of recommended models.
type Catalog struct {
	Models []CatalogModel `json:"models"`
}

// LoadCatalog reads the model catalog from the embedded data file.
func LoadCatalog() ([]CatalogModel, error) {
	// Try loading from executable directory or from repo
	paths := []string{
		"data/models.json",
		"~/.clauncher/models.json",
	}
	for _, p := range paths {
		if p == "~/.clauncher/models.json" {
			home, _ := os.UserHomeDir()
			p = filepath.Join(home, ".clauncher", "models.json")
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cat Catalog
		if err := json.Unmarshal(data, &cat); err != nil {
			continue
		}
		return cat.Models, nil
	}
	return nil, fmt.Errorf("no model catalog found")
}

// IsModelDownloaded checks if the HF repo already has blob files in the cache.
func IsModelDownloaded(hfRepo string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Normalize repo: replace / with -- for HF cache path
	cachePath := strings.ReplaceAll(hfRepo, "/", "--")
	if idx := strings.LastIndex(cachePath, ":"); idx != -1 {
		cachePath = cachePath[:idx]
	}

	blobDir := filepath.Join(home, ".cache", "huggingface", "hub", fmt.Sprintf("models--%s", cachePath), "blobs")
	entries, err := os.ReadDir(blobDir)
	if err != nil {
		return false
	}

	// Model is downloaded if blobs exist and none are .downloadInProgress
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".downloadInProgress") {
			return false
		}
		if !e.IsDir() {
			return true
		}
	}
	return false
}

// DownloadModel downloads a model from HuggingFace using llama.
func DownloadModel(ctx context.Context, hfRepo string) error {
	llamaPath, err := exec.LookPath("llama")
	if err != nil {
		return fmt.Errorf("llama not found in PATH — install llama.cpp to download models")
	}

	// Check if model is already downloaded
	if IsModelDownloaded(hfRepo) {
		log.Printf("[download] %s already exists, skipping", hfRepo)
		return nil
	}

	log.Printf("[download] binary: %s, repo: %s", llamaPath, hfRepo)

	cmd := exec.CommandContext(ctx, llamaPath, "download", "-hf", hfRepo)
	log.Printf("[download] command: %s", cmd.String())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}

	var stderrBuf strings.Builder
	var wg sync.WaitGroup

	wg.Add(2)

	// Discard stdout (download progress goes to stderr)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stdout)
	}()

	// Stream stderr for logging
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[download] %s", line)
			stderrBuf.WriteString(line + "\n")
		}
		if scanner.Err() != nil {
			log.Printf("[download] stderr error: %v", scanner.Err())
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		log.Printf("[download] cancelling download (reason: %v)", ctx.Err())
		cmd.Process.Kill()
		<-errChan
		wg.Wait()
		return fmt.Errorf("download cancelled or timed out: %w", ctx.Err())
	case err = <-errChan:
		wg.Wait()
		if err != nil {
			log.Printf("[download] failed: %v", err)
			log.Printf("[download] stderr: %s", stderrBuf.String())
			return fmt.Errorf("download failed: %w (stderr: %s)", err, stderrBuf.String())
		}
		log.Printf("[download] completed successfully: %s", hfRepo)
		return nil
	}
}
// GPUStats holds structured GPU metrics.
type GPUStats struct {
	Temperature float64 // °C (junction/edge)
	GPUUsage    float64 // % compute utilization
	MemoryUsage float64 // % VRAM used
	UsageText   string  // fallback text when structured parsing isn't available
}

// GetGPUStats returns GPU metrics from available tools.
func GetGPUStats() GPUStats {
	var stats GPUStats

	// Try ROCm (AMD) first
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		cmd := exec.Command("rocm-smi", "--showtemp", "--showpower", "--showmemuse", "--showuse", "--json")
		if output, err := cmd.Output(); err == nil {
			stats = parseROCmOutput(output)
			if stats.Temperature > 0 {
				return stats
			}
		}
	}

	// Try NVIDIA
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.used,memory.total,temperature.gpu,utilization.gpu", "--format=csv,noheader,nounits")
		if output, err := cmd.Output(); err == nil {
			stats = parseNvidiaOutput(output)
			if stats.Temperature > 0 {
				return stats
			}
		}
	}

	// Try Vulkan info via llama --version
	if _, err := exec.LookPath("llama"); err == nil {
		cmd := exec.Command("llama", "--version")
		if output, err := cmd.Output(); err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "vulkan") || strings.Contains(lower, "gpu") {
					stats.UsageText = strings.TrimSpace(line)
					return stats
				}
			}
		}
	}

	return stats
}

func parseROCmOutput(output []byte) GPUStats {
	var result GPUStats
	var data map[string]map[string]string
	if err := json.Unmarshal(output, &data); err != nil {
		return result
	}

	for _, card := range data {
		// Temperature (use junction, fall back to edge)
		if v := card["Temperature (Sensor junction) (C)"]; v != "" {
			if t, err := strconv.ParseFloat(v, 64); err == nil && t > 0 {
				result.Temperature = t
			}
		} else if v := card["Temperature (Sensor edge) (C)"]; v != "" {
			if t, err := strconv.ParseFloat(v, 64); err == nil && t > 0 {
				result.Temperature = t
			}
		}

		// GPU compute usage
		if v := card["GPU use (%)"]; v != "" {
			if u, err := strconv.ParseFloat(v, 64); err == nil {
				result.GPUUsage = u
			}
		}

		// VRAM usage
		if v := card["GPU Memory Allocated (VRAM%)"]; v != "" {
			if m, err := strconv.ParseFloat(v, 64); err == nil {
				result.MemoryUsage = m
			}
		}
	}

	return result
}

func parseNvidiaOutput(output []byte) GPUStats {
	var result GPUStats
	line := strings.TrimSpace(string(output))
	if line == "" {
		return result
	}

	parts := strings.Split(line, ",")
	if len(parts) < 5 {
		return result
	}

	// Skip GPU name (parts[0])
	// parts[1] = memory.used MB, parts[2] = memory.total MB
	memUsed, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	memTotal, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if memTotal > 0 {
		result.MemoryUsage = (memUsed / memTotal) * 100
	}

	// parts[3] = temperature
	if t, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
		result.Temperature = t
	}

	// parts[4] = utilization.gpu
	if u, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err == nil {
		result.GPUUsage = u
	}

	return result
}
