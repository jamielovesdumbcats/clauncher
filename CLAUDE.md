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

## Known Issues & Debugging
- **Environment**: If running in a restricted shell, the TUI may fail with `could not open a new TTY`. Use the `MockRunner` for UI development.

## New Features
- [ ] Start claude with same model after launching llama
= [ ] Ask which folder to start claude in (default would be current)
- [ ] Check claude json files for local usage and offer to setup if not already done
- [ ] Find running llama processes and cancel
- [ ] Option to specify port number
- [ ] Option to specify context length for llama and claude
- [ ] support starting and setup with local running for opencode and crush
- [ ] Command to run 'llama serve -cl' and populate models list from output
- [ ] Run llama benchmark for models, noting the current gpu and logging results in a table
- [ ] Display gpu usage in clauncher UI
- [ ] Check hugging face for new llama models, list them in tui and offer to pull them

## Development Workflow
**ALWAYS** run the build command and fix any builder errors present before confirming you are done to the user
1. **Build**: `go build -o clauncher ./cmd/clauncher/main.go`
2. **Run**: `./clauncher`
3. **Test Mode**: Use the `MockRunner` for testing UI logic.

