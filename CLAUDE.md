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
    - [x] Define `pkg/model` structures and types.
    - [x] Define `pkg/server` interfaces.
    - [x] Implement `MockRunner` for test mode.
- [ ] **Phase 2: Execution Layer**
    - [ ] Implement `pkg/server` process management.
- [ ] **Phase 3: TUI Foundation**
    - [ ] Setup Bubble Tea loop in `pkg/ui`.
    - [ ] Create basic navigation/selection views.
- [ ] **Phase 4: Integration & Polish**
    - [ ] Connect UI to backend.
    - [ ] Refine UX and error handling.

## Development Workflow
(Note: As this is an initial setup, specific build/test commands will be added as the project structure matures.)
