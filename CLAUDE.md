# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Aim
A Terminal User Interface (TUI) to manage and launch Claude Code CLI and Llama CPP with various local models.

## Technology Stack
- **Language**: Go
- **TUI Libraries**: Charmbracelet ecosystem (Bubble Tea, Bubbles, Lip Gloss, Huh)

## High-Level Architecture
The project is organized into the following structure:
- `cmd/clauncher/`: Main application entry point.
- `pkg/`: Core logic and domain models.
    - `model/`: Model definitions and management logic.
    - `server/`: Logic for interacting with Llama CPP/inference servers.
    - `ui/`: TUI component implementations.
- `ui/`: General UI assets and configurations.

## Implementation Progress
- [x] **Phase 1: Core Domain & Interfaces**
- [x] **Phase 2: Execution Layer**
- [x] **Phase 3: TUI Foundation**
- [x] **Phase 4: Feature Integration**
    - [x] Integrate `CommandRunner` with the UI.
    - [x] Implement real-time log streaming in the dashboard.
    - [x] Implement process control (Start/Stop) from the TUI.
    - [x] Add error handling/crash detection UI.
- [x] **Phase 5: Dynamic Model Discovery**
    - [x] Auto-discover locally installed models via `llama serve -cl`
    - [x] Real-time model list refresh from TUI
    - [x] Fixed "signal: killed" error on intentional stop

## Known Issues & Debugging
- **Environment**: If running in a restricted shell, the TUI may fail with `could not open a new TTY`. Use the `MockRunner` for UI development.

## Launching claude with local

From [this page](https://jonathansblog.co.uk/using-claude-code-with-local-llm-models-the-complete-guide) the steps for local claude usage should be as follows

Start the llama server

llama-server 
  --model "$MODEL_PATH" 
  --alias "my-model" 
  --temp 1.0 
  --top-p 0.95 
  --port 8001 
  --ctx-size 131072 
  --flash-attn on

Point Claude Code at it
export ANTHROPIC_BASE_URL=https://localhost:8081 
or a preferred port number, llama default is 8080 but we are using 8081 while testing as 8080 is already taken

Fix the KV Cache Performance Issue

Edit (or create) ~/.claude/settings.json and add:
{
  "env": {
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}

Launch claude with
claude --model my-model (eg model defined and launched with llama earlier)

## Launching opencode with local

From [this page](https://dev.to/ferdousulhaque/opencode-for-agentic-development-with-local-llms-2h4k) it looks like we need the following

check for .config/opencode/opencode.json and create if not present

if creating put the following in

{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "llama-cpp": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "llama-cpp (local)",
      "options": {
        "baseURL": "http://localhost:8081/v1"
      },
      "models": {
        "qwen3:8b": {
          "name": "qwen3:8b"
        }
      }
    }
  }
}

but we use the port we've setup, we should verify this is correct when starting opencode after the user has changed the port in config

if the json already exists we need to insert the provider into the existing config.

finally we check which directory to use, cd to it and launch with opencode

crush seems to require a larger context than 4096 so we should warn if context set is smaller than that for the model and offer the user the chance to change it, providing them with the max context proved by the model

## Launching crush with local

From [this page](https://soc.meschbach.com/posts/2026/01/12-experiments-with-crush-and-ollama--qwen-3-coder/) it looks like we need the following

check for ~/.config/crush/crush.json and create if not present

if creating put the following in

{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "llama-cpp": {
      "name": "llama-cpp",
      "base_url": "http://localhost:8081/v1/",
      "type": "openai",
      "models": [
        {
        "name": "Qwen 3 8B",
        "id": "qwen3:8b-ctx-40960",
        "context_window": 40960
        }
      ]
    }
  },
  "options": {
    "disable_metrics": true
  }
}

but we use the port we've setup, we should verify this is correct when starting crush after the user has changed the port in config

if the json already exists we need to insert the provider into the existing config.

finally we check which directory to use, cd to it and launch with crush

## New Features
- [x] Start claude with same model after launching llama
= [x] Ask which folder to start claude in (default would be current)
- [ ] Check claude json files for local usage and offer to setup if not already done
- [ ] Find running llama processes and cancel
- [x] Option to specify port number
- [x] Option to specify context length for llama and claude
- [x] support starting and setup with local running for opencode and crush
- [x] Command to run 'llama serve -cl' and populate models list from output
- [x] Run llama benchmark for models, noting the current gpu and logging results in a table
- [ ] Display gpu usage in clauncher UI
- [ ] Check hugging face for new llama models, list them in tui and offer to pull them
- [ ] run llama fit-params -hf modelname store details with benchmark data and suggest when launching model. command returns -c 4096 -ngl 0
- [ ] run llama perplexity -hf modelname store details with benchmark data and display with benchmarking data. command return currently unknown

## Development Workflow
**ALWAYS** run the build command and fix any builder errors present before confirming you are done to the user
1. **Build**: `go build -o clauncher ./cmd/clauncher/main.go`
2. **Run**: `./clauncher`
3. **Test Mode**: Use the `MockRunner` for testing UI logic.

