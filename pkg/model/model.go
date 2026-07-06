package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ModelType string

const (
	ClaudeCode ModelType = "claude-code"
	LlamaCPP   ModelType = "llama-cpp"
)

type LaunchOption int

const (
	LaunchLlamaServer LaunchOption = iota
	LaunchLlamaCLI
	LaunchClaudeCode
	LaunchOpencode
	LaunchCrush
)

type ProcessStatus string

const (
	StatusRunning  ProcessStatus = "running"
	StatusStopped  ProcessStatus = "stopped"
	StatusCrashed  ProcessStatus = "crashed"
	StatusStarting ProcessStatus = "starting"
)

type UsageMetrics struct {
	TokensUsed int64
	LastUsed   time.Time
}

type Model struct {
	ID     string
	Name   string
	Type   ModelType
	Config map[string]string
	Usage  UsageMetrics
}

type ModelManager interface {
	GetModels() []Model
	GetModelByID(id string) (Model, bool)
	AddModel(m Model) error
	RemoveModel(id string) error
}

type BenchmarkResult struct {
	ModelName string `json:"model_name"`
	Timestamp string `json:"timestamp"`

	MQTTokensPerSecond float64 `json:"mqq_tps"`
	MQSTotalTimeMs     float64 `json:"mqs_total_ms"`

	PPTokensPerSecond float64 `json:"pp_tps"`
	PPTotalTimeMs     float64 `json:"pp_total_ms"`

	SGTTokensPerSecond float64 `json:"sgt_tps"`
	SGTTotalTimeMs     float64 `json:"sgt_total_ms"`
}

// BenchmarkStore manages persistence of benchmark results.
type BenchmarkStore struct {
	FilePath string
}

func NewBenchmarkStore(path string) *BenchmarkStore {
	return &BenchmarkStore{FilePath: path}
}

func (s *BenchmarkStore) DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clauncher", "benchmarks.json")
}

func (s *BenchmarkStore) Load() ([]BenchmarkResult, error) {
	if s.FilePath == "" {
		s.FilePath = s.DefaultPath()
	}
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []BenchmarkResult{}, nil
		}
		return nil, err
	}
	var results []BenchmarkResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *BenchmarkStore) Save(results []BenchmarkResult) error {
	if s.FilePath == "" {
		s.FilePath = s.DefaultPath()
	}
	dir := filepath.Dir(s.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.FilePath, data, 0o644)
}

func (s *BenchmarkStore) Add(result BenchmarkResult) error {
	results, err := s.Load()
	if err != nil {
		return err
	}
	results = append(results, result)
	return s.Save(results)
}

func (s *BenchmarkStore) GetLatest(modelName string) (BenchmarkResult, bool) {
	results, err := s.Load()
	if err != nil {
		return BenchmarkResult{}, false
	}
	var latest BenchmarkResult
	found := false
	for _, r := range results {
		if r.ModelName == modelName {
			latest = r
			found = true
		}
	}
	return latest, found
}
