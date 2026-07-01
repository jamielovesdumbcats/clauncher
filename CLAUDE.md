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
- [ ] **Phase 4: Feature Integration**
    - [ ] Integrate `CommandRunner` with the UI.
    - [ ] Implement real-time log streaming in the dashboard.
    - [ ] Implement process control (Start/Stop) from the TUI.
    - [ ] Add error handling/crash detection UI.

## Known Issues & Debugging
- **Build Error**: Type mismatch in `pkg/ui/model.go`. Ensure `StatusUpdateMsg` uses `model.ProcessStatus` (from `pkg/model`) instead of `server.ProcessStatus`.
- **Environment**: If running in a restricted shell, the TUI may fail with `could not open a new TTY`. Use the `MockRunner` for UI development.

## Development Workflow
1. **Build**: `go build -o clauncher ./cmd/clauncher/main.go`
2. **Run**: `./clauncher`
3. **Test Mode**: Use the `MockRunner` for testing UI logic.
