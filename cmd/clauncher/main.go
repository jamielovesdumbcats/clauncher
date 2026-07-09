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

	// Check for running llama servers
	runtimeServers, _ := server.FindRunningLlamaServers()
	if len(runtimeServers) > 0 {
		fmt.Printf("Found %d running llama server(s)\n", len(runtimeServers))
	}

	// Load model catalog
	catalog, err := server.LoadCatalog()
	if err != nil {
		fmt.Printf("Warning: Could not load model catalog: %v\n", err)
		catalog = []server.CatalogModel{}
	} else {
		fmt.Printf("Loaded %d catalog models\n", len(catalog))
	}

	// Initialize the runner with the Llama builder
	runner := server.NewCommandRunner(LlamaCPPCommandBuilder)

	// Initialize the UI app with the dynamic model list
	app := ui.NewApp(localModels, runner, runtimeServers, catalog)

	// Start the Bubble Tea program
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
	}
}
