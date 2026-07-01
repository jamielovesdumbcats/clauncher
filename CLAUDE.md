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

## Development Workflow
1. **Build**: `go build -o clauncher ./cmd/clauncher/main.go`
2. **Run**: `./clauncher`
3. **Test Mode**: Use the `MockRunner` (default in development) to test UI logic without needing live processes.
