# Clauncher — Agent Guide

## What It Is

A Go TUI (Terminal User Interface) for launching and managing local LLM inference via **llama.cpp**, and launching AI-powered CLI tools (Claude Code, Opencode, Crush) against those local models.

Built with the [Charmbracelet](https://charm.land) ecosystem: Bubble Tea (TUI framework), Lip Gloss, Bubbles, Huh.

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
2. **Selection View**: User picks a model → transitions to **Launch Options View**.
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
- View states: `ViewSelection`, `ViewDashboard`, `ViewLaunchOptions`, `ViewBenchmark`, `ViewCatalog`, `ViewSearch`, `ViewQuants`, `ViewKillServer`.
- Errors are stored in `a.err` and displayed at the top of `View()`. Clear `a.err` on navigation.

### Search & Quant Views

- `ViewSearch`: triggered by `s` key. Uses a `textinput` for query. Results show repo IDs with stars and size. Arrow keys navigate results, Enter adds selected model to catalog (blurs input first). `b`/Esc returns to selection.
- `ViewQuants`: triggered by Enter on a search result. Lists GGUF quant files from the repo's `main` branch. Arrow keys navigate, Enter adds to catalog. `b`/Esc returns to search (restores focus to search input).
- Key routing: `Update()` pre-switches on `ViewSearch`/`ViewQuants` to intercept arrow/enter/back/quit keys, routing all other keys to the textinput.

### Launch Methods (CLI, Claude, Opencode, Crush)

- `launchLlamaCLI`: tries terminal emulators (gnome-terminal, kitty, alacritty, etc.), falls back to foreground `sh -c`.
- `launchClaudeCode` / `launchOpencode` / `launchCrush`: all follow the same pattern — start llama server, `time.Sleep(2s)`, write config JSON, launch app.
- Config merging is implemented for existing config files.
- These methods run in a `tea.Cmd` closure — any `exec.Command` errors must return a `messages.ErrorMsg`.

### Model Discovery

- `ListLocalModels()` runs `llama serve -cl` with a timeout and parses numbered output lines.
- Model IDs are sanitized (`/` → `-`). Display names strip `-GGUF` suffixes.
- The full HF path with quant (e.g., `mradermacher/gemma-4-26B-A4B-it-GGUF:IQ4_XS`) is stored in `Config["model_name"]`.

### Signal Handling

- Intentional stops via `runner.Stop()` cancel the context. `monitorExit()` checks `procCtx.Err() != nil` to distinguish from crashes.
- The "signal: killed" issue was fixed by treating context-cancellation exits as `StatusStopped`, not `StatusCrashed`.

### Process.Release()

- After `cmd.Start()` in launch methods, call `cmd.Process.Release()` to avoid goroutine leaks.
- **Always call `Start()` before accessing `cmd.Process`** — nil pointer panics otherwise (was a real bug in `launchLlamaCLI` fallback path).

---

## Launch Config Details

### Claude Code
- Start llama server with model, port, context size, flash-attn.
- Set `ANTHROPIC_BASE_URL=https://localhost:<port>` and run `claude --model my-model`.
- Auto-setup `~/.claude/settings.json` with `"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"` for KV cache performance.

### Opencode
- Config: `~/.config/opencode/opencode.json`. Provider key: `"llama-cpp"` under `"provider"`.
- Base URL format: `http://localhost:<port>/v1`.
- If config exists, merge the provider into existing JSON.
- Ask user which working directory to launch in.

### Crush
- Config: `~/.config/crush/crush.json`. Provider key: `"llama-cpp"` under `"providers"`.
- Base URL format: `http://localhost:<port>/v1/` (trailing slash matters).
- If config exists, merge the provider into existing JSON.
- Crush requires context >= 4096 — warn user if model context is smaller, offer to change.
- Ask user which working directory to launch in.

---

## Implemented Features

- [x] HuggingFace model search — search repos, browse quants, add to catalog
- [x] Search UX — auto-blur input after results, focus restore on back nav, spinner during search
- [x] Model catalog in user config (`~/.clauncher/models.json`)
- [x] Catalog download status indicators (✓/✗)
- [x] Search key (`s`) in selection view bottom bar
- [x] Arrow key navigation for model and option selection
- [x] Configurable port and context length via UI
- [x] Find and kill existing llama processes before starting new ones
- [x] Merge existing app configs (Opencode, Crush) instead of skipping
- [x] Ask which working directory to launch Claude/opencode/crush in
- [x] Context length warnings for Crush/opencode (< 4096) with option to change
- [x] Pre-configured model list for downloads (stored in repo)
- [x] Model benchmarking
- [x] Auto-setup `~/.claude/settings.json` for local usage (needs testing)

---

## Feature Backlog


### TODO
- [ ] UI enhancements — more needed to improve look using Charm libraries


### Future
- [ ] Ollama as alternative backend

---

## Known Issues & Debugging

- **Environment**: If running in a restricted shell, the TUI may fail with `could not open a new TTY`. Use the `MockRunner` for UI development.

---

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — terminal styling
- `github.com/charmbracelet/bubbles` — reusable TUI components
- `github.com/charmbracelet/huh` — form/input components
- No external HTTP libraries, databases, or config parsers yet (all config is raw `os.ReadFile` + `json.MarshalIndent`).
