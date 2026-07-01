package model

import (
	"time"
)

type ModelType string

const (
	ClaudeCode ModelType = "claude-code"
	LlamaCPP   ModelType = "llama-cpp"
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
