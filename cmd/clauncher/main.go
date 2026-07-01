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

	// Initialize the UI app
	app := ui.NewApp([]model.Model{m}, runner)

	// Start the Bubble Tea program
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
	}
}
