package main

import (
	"fmt"
	//"os/exec"
	//"os"

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
	fmt.Println("Clauncher starting (Real Runner Test)...")

	// 1. DEBUG: Print the command we are about to run to the terminal
	m := model.Model{
		ID:   "test-model",
		Name: "Test Model",
		Type: model.LlamaCPP,
		Config: map[string]string{
			"model_name": "mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS",
		},
	}

	cmdName, cmdArgs := LlamaCPPCommandBuilder(m)
	fmt.Printf("\n[DEBUG] Attempting to run command: %s %v\n", cmdName, cmdArgs)
	fmt.Println("[DEBUG] Check if this command works directly in your terminal first!\n")

	// 2. Initialize the runner with the Llama builder
	runner := server.NewCommandRunner(LlamaCPPCommandBuilder)

	// 3. Initialize the UI app
	app := ui.NewApp([]model.Model{m}, runner)

	// 4. Start the Bubble Tea program
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
	}
}
