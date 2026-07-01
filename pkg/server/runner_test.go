package server

import (
	"context"
	"testing"
	"time"

	"clauncher/pkg/model"
)

func TestMockRunner(t *testing.T) {
	m := model.Model{
		ID:   "test-model",
		Name: "Test Model",
		Type: model.LlamaCPP,
	}
	runner := NewMockRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logChan, err := runner.Start(ctx, m)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Check if we get logs
	select {
	case msg := <-logChan:
		if msg == "" {
			t.Error("received empty log message")
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for log")
	}

	// Test stop
	err = runner.Stop()
	if err != nil {
		t.Errorf("failed to stop: %v", err)
	}

	status := runner.Status()
	if status.Status != model.StatusStopped {
		t.Errorf("expected status %s, got %s", model.StatusStopped, status.Status)
	}
}
