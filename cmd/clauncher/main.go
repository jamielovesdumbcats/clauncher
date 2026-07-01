package main

import (
	"context"
	"fmt"
	"time"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
)

// LlamaCPPCommandBuilder builds the command for llama serve
func LlamaCPPCommandBuilder(m model.Model) (string, []string) {
	// For development/test, we'll use a mock command if possible,
	// but here we define the real one.
	// Example: llama serve -hf mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS
	return "llama", []string{"serve", "-hf", m.Config["model_name"]}
}

func main() {
	fmt.Println("Clauncher starting (Real Runner Test)...")

	// Define a mock model with configuration
	m := model.Model{
		ID:   "test-model",
		Name: "Test Model",
		Type: model.LlamaCPP,
		Config: map[string]string{
			"model_name": "mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS",
		},
	}

	// Initialize the runner with the Llama builder
	runner := server.NewCommandRunner(LlamaCPPCommandBuilder)

	// Start the process
	// Using a context that we can cancel
	// In a real app, this would be managed by the TUI lifecycle
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Printf("Launching model: %s\n", m.Name)
	logChan, err := runner.Start(ctx, m)
	if err != nil {
		fmt.Printf("Error starting: %v\n", err)
		return
	}

	// Listen to logs in a separate goroutine
	done := make(chan struct{})
	go func() {
		for msg := range logChan {
			fmt.Printf("[LOG]: %s\n", msg)
		}
		close(done)
	}()

	// Wait for the context to expire (simulating a timeout or manual stop)
	<-ctx.Done()
	fmt.Println("Context expired or finished. Stopping process...")
	runner.Stop()

	// Wait for log channel to be closed
	<-done

	status := runner.Status()
	fmt.Printf("Final Status: %s\n", status.Status)
	if status.Error != nil {
		fmt.Printf("Error: %v\n", status.Error)
	}
}
