package server

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clauncher/pkg/model"
)

// RunBenchmark executes llama bench for the given model and returns the parsed results.
func RunBenchmark(ctx context.Context, m model.Model) (*model.BenchmarkResult, error) {
	// Check if llama is available
	if _, err := exec.LookPath("llama"); err != nil {
		return nil, fmt.Errorf("llama not found in PATH — install llama.cpp to run benchmarks")
	}

	// Run llama bench with the model
	cmd := exec.CommandContext(ctx, "llama", "bench", "-hf", m.Config["model_name"])
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("benchmark failed: %w (output: %s)", err, output)
	}

	return parseBenchmarkOutput(string(output), m), nil
}

// parseBenchmarkOutput extracts key metrics from llama bench output
func parseBenchmarkOutput(output string, m model.Model) *model.BenchmarkResult {
	result := &model.BenchmarkResult{
		ModelName: m.Name,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Extract tokens per second values using regex
	// Pattern: "xxx t/s" or similar
	tpsPattern := regexp.MustCompile(`([\d.]+)\s*t/s`)
	totalTimePattern := regexp.MustCompile(`([\d.]+)\s*ms`)

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Multi-query sampling throughput
		if strings.Contains(strings.ToLower(line), "multi-query") && strings.Contains(strings.ToLower(line), "sampling") {
			matches := tpsPattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				val, _ := strconv.ParseFloat(matches[1], 64)
				result.MQTTokensPerSecond = val
			}
			// Check next lines for total time
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				matches = totalTimePattern.FindStringSubmatch(nextLine)
				if len(matches) > 1 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					result.MQSTotalTimeMs = val
				}
			}
		}

		// Prompt processing speed
		if strings.Contains(strings.ToLower(line), "prompt") && strings.Contains(strings.ToLower(line), "processing") {
			matches := tpsPattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				val, _ := strconv.ParseFloat(matches[1], 64)
				result.PPTokensPerSecond = val
			}
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				matches = totalTimePattern.FindStringSubmatch(nextLine)
				if len(matches) > 1 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					result.PPTotalTimeMs = val
				}
			}
		}

		// Single token generation speed
		if strings.Contains(strings.ToLower(line), "single") && strings.Contains(strings.ToLower(line), "token") && strings.Contains(strings.ToLower(line), "generation") {
			matches := tpsPattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				val, _ := strconv.ParseFloat(matches[1], 64)
				result.SGTTokensPerSecond = val
			}
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				matches = totalTimePattern.FindStringSubmatch(nextLine)
				if len(matches) > 1 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					result.SGTTotalTimeMs = val
				}
			}
		}
	}

	return result
}
