package main

import (
	"context"
	"fmt"
	"time"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
)

func main() {
	fmt.Println("Clauncher starting...")

	// Initialize Mock Runner for development
	runner := server.NewMockRunner()

	// Define a mock model
	m := model.Model{
		ID:   "test-model",
		Name: "Test Model",
		Type: model.LlamaCPP,
	}

	fmt.Printf("Running test with model: %s\n", m.Name)

	// Start the process
	// In a real app, this would be managed by the TUI lifecycle
	logChan, err := runner.Start(context.Background(), m)
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

	// Let it run for 10 seconds
	fmt.Println("Running for 10 seconds...")
	time.Sleep(10 * time.Second)

	fmt.Println("Stopping process...")
	if err := runner.Stop(); err != nil {
		fmt.Printf("Error stopping: %v\n", err)
	}

	// Wait for log channel to close
	<-done

	status := runner.Status()
	fmt.Printf("Final Status: %s\n", status.Status)
}
