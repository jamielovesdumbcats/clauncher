# Clauncher — Agent Guide

## What It Is

A Go TUI (Terminal User Interface) for launching and managing local LLM inference via **llama.cpp**, and launching AI-powered CLI tools (Claude Code, Opencode, Crush) against those local models.

Built with the [Charmbracelet](https://charm.land) ecosystem: Bubble Tea (TUI framework), Lip Gloss (styling).

---

## Commands

| Action | Command |
|--------|---------|
| Build | `go build -o clauncher ./cmd/clauncher/main.go` |
| Run | `./clauncher` |
| Test | `go test ./...` |

**Always run the build command after code changes** to verify before telling the user you're done.

---

## Architecture

```
cmd/clauncher/main.go          — entry point: discovers models, wires runner + UI
pkg/model/model.go             — domain types (Model, LaunchOption, ProcessStatus)
pkg/server/command_runner.go   — real process lifecycle (start/stop/logs) + ListLocalModels()
pkg/server/runner_test.go      — tests against MockRunner
pkg/ui/model.go                — Bubble Tea App model (views, update, launch methods)
pkg/ui/messages/messages.go    — Bubble Tea message types
pkg/ui/theme/theme.go          — Lip Gloss color/style definitions
```

### Control Flow

1. **Startup**: `main.go` calls `server.ListLocalModels()` (runs `llama serve -cl`), creates a `CommandRunner`, passes both to `ui.NewApp()`.
2. **Selection View**: User picks a model by number → transitions to **Launch Options View**.
3. **Launch Options**: User picks how to launch (Server, CLI, Claude Code, Opencode, Crush) → `LaunchOptionSelectedMsg` is dispatched.
4. **Dispatch** (in `App.Update`):
   - `LaunchLlamaServer` → dashboard view + `startProcess()` (managed via CommandRunner)
   - `LaunchLlamaCLI` → opens external terminal (or fallback to current terminal)
   - `LaunchClaudeCode` / `LaunchOpencode` / `LaunchCrush` → starts llama server in background, writes app-specific config, then launches the app
5. **Dashboard View**: Shows process status, streaming logs, start/stop controls.

### Process Lifecycle (`CommandRunner`)

- `Start()` creates `exec.CommandContext` with a cancellable context, pipes stdout/stderr, spawns `monitorPipes()` and `monitorExit()` goroutines.
- Logs flow through a buffered channel (`logChan`, size 100) — non-blocking sends, drops lines if full.
- `Stop()` cancels the context. `monitorExit()` detects context cancellation vs. crash and sets `StatusStopped` or `StatusCrashed`.
- `ClearError()` resets error state after intentional stops (called from `toggleProcess`).

---

## Key Patterns & Gotchas

### Bubble Tea Model

- `App` is the root `tea.Model`. Views are simple string renderers; state transitions happen in `Update()` via `currentView`.
- Three view states: `ViewSelection`, `ViewDashboard`, `ViewLaunchOptions`.
- Errors are stored in `a.err` and displayed at the top of `View()`. Clear `a.err` on navigation.

### Launch Methods (CLI, Claude, Opencode, Crush)

- `launchLlamaCLI`: tries terminal emulators (gnome-terminal, kitty, alacritty, etc.), falls back to foreground `sh -c`.
- `launchClaudeCode` / `launchOpencode` / `launchCrush`: all follow the same pattern — start llama server, `time.Sleep(2s)`, write config JSON, launch app.
- **Config merging is TODO**: if the config file already exists, current code skips merge (`_ = data`). Future work should parse and inject the provider into existing JSON.
- These methods run in a `tea.Cmd` closure — any `exec.Command` errors must return a `messages.ErrorMsg`.

### Model Discovery

- `ListLocalModels()` runs `llama serve -cl` with a 10s timeout and parses numbered output lines.
- Model IDs are sanitized (`/` → `-`). Display names strip `-GGUF` suffixes.
- The full HF path with quant (e.g., `mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS`) is stored in `Config["model_name"]`.

### Signal Handling

- Intentional stops via `runner.Stop()` cancel the context. `monitorExit()` checks `procCtx.Err() != nil` to distinguish from crashes.
- The "signal: killed" issue was fixed by treating context-cancellation exits as `StatusStopped`, not `StatusCrashed`.

### Process.Release()

- After `cmd.Start()` in launch methods, call `cmd.Process.Release()` to avoid goroutine leaks.
- **Always call `Start()` before accessing `cmd.Process`** — nil pointer panics otherwise (was a real bug in `launchLlamaCLI` fallback path).

---

## Launch Config Details (from CLAUDE.md)

### Claude Code
- Set `ANTHROPIC_BASE_URL=https://localhost:<port>` and run `claude --model my-model`.
- Add `"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"` to `~/.claude/settings.json` for KV cache performance.

### Opencode
- Config: `~/.config/opencode/opencode.json`. Provider key: `"llama-cpp"` under `"provider"`.
- Base URL format: `http://localhost:<port>/v1`.

### Crush
- Config: `~/.config/crush/crush.json`. Provider key: `"llama-cpp"` under `"providers"`.
- Base URL format: `http://localhost:<port>/v1/` (trailing slash matters).
- Crush requires context >= 4096 — warn user if model context is smaller.

---

## Feature Backlog

See `CLAUDE.md` → "New Features" for the tracked checklist. Highlights:

- [ ] Ask which working directory to launch Claude/opencode/crush in
- [ ] Auto-setup `~/.claude/settings.json` for local usage
- [ ] Find and kill existing llama processes before starting new ones
- [ ] Configurable port and context length via UI
- [ ] Context length warnings for Crush/opencode (< 4096)
- [ ] Merge existing app configs (Opencode, Crush) instead of skipping
- [ ] GPU usage display
- [ ] Model benchmarking
- [ ] HuggingFace model discovery
- [ ] Ollama as alternative backend

---

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — terminal styling
- No external HTTP libraries, databases, or config parsers yet (all config is raw `os.ReadFile` + `json.MarshalIndent`).
