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

	// Parse markdown table rows: | model | size | params | backend | ngl | test | t/s |
	// Example: | gemma4 26B ... | 13.08 GiB | 25.23 B | Vulkan | -1 | pp512 | 79.27 ± 5.05 |
	tsPattern := regexp.MustCompile(`\|\s*([\d.]+)\s*(?:±\s*[\d.]+)?\s*\|$`)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}

		// fields[0] is empty (leading |), fields[-1] is empty (trailing |)
		// fields[-2] = t/s value, fields[-3] = test name
		fields := strings.Split(line, "|")
		if len(fields) < 8 {
			continue
		}

		tsName := strings.TrimSpace(fields[len(fields)-3])
		tsName = strings.ToLower(tsName)

		matches := tsPattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}

		val, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			continue
		}

		if strings.HasPrefix(tsName, "pp") {
			result.PPTokensPerSecond = val
		} else if strings.HasPrefix(tsName, "tg") || strings.HasPrefix(tsName, "sg") {
			result.SGTTokensPerSecond = val
		} else if strings.HasPrefix(tsName, "mq") {
			result.MQTTokensPerSecond = val
		}
	}
	return result
}
