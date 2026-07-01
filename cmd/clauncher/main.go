package main

import (
	"fmt"

	"clauncher/pkg/model"
	"clauncher/pkg/server"
	"clauncher/pkg/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// LlamaCPPCommandBuilder builds the command for llama serve
func LlamaCPPCommandBuilder(m model.Model) (string, []string) {
	// For development/test, we'll use a command if possible,
	// but here we define the real one.
	// Example: llama serve -hf mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS
	return "llama", []string{"serve", "-hf", m.Config["model_name"], "--port", "8081"}
}

func main() {
	fmt.Println("Clauncher starting...")

	// Fetch locally installed models
	localModels, err := server.ListLocalModels()
	if err != nil {
		fmt.Printf("Warning: Could not fetch local models: %v\n", err)
		fmt.Println("Starting with empty model list. Press 'r' to refresh.")
		localModels = []model.Model{}
	} else {
		fmt.Printf("Found %d local models\n", len(localModels))
	}

	// Initialize the runner with the Llama builder
	runner := server.NewCommandRunner(LlamaCPPCommandBuilder)

	// Initialize the UI app with the dynamic model list
	app := ui.NewApp(localModels, runner)

	// Start the Bubble Tea program
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
	}
}