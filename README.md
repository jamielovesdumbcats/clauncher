# Clauncher

A powerful Terminal User Interface (TUI) for launching and managing local LLM inference via [llama.cpp](https://github.com/ggerganov/llama.cpp), and launching AI-powered CLI tools (Claude Code, Opencode, Crush) against those local models.

Built with the [Charmbracelet](https://charm.land) ecosystem: Bubble Tea, Lip Gloss, Bubbles, and Huh.

## ✨ Features

- **Dynamic Model Discovery**: Automatically detects all locally installed LLM models via `llama serve -cl`.
- **HuggingFace Search**: Search the HuggingFace hub directly from the TUI, browse GGUF quant files, and add models to your catalog.
- **Model Catalog**: Pre-configured model list persisted in `~/.clauncher/models.json` with download status indicators.
- **Multiple Launch Modes**: Launch models as a server, CLI, or with AI tools (Claude Code, Opencode, Crush).
- **Configurable Launch Settings**: Adjust port and context length per launch via the UI.
- **Server Management**: Find and kill existing llama servers before starting new ones.
- **Real-time GPU Stats**: Live ROCm or NVIDIA GPU metrics panel showing temperature, memory, and utilization.
- **Real-time Logs**: Monitor model output and server status directly within the interface.
- **Model Benchmarking**: Benchmark models for tokens/second and memory usage.
- **Process Control**: Start, stop, and manage the lifecycle of your running models.
- **Auto-Config for AI Tools**: Automatically sets up config files for Claude Code, Opencode, and Crush to connect to your local llama server.
- **Developer Friendly**: Includes a built-in `MockRunner` for rapid UI development and testing.

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/) (1.21 or later recommended)
- [llama.cpp](https://github.com/ggerganov/llama.cpp) installed and on your `PATH`

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/jasonjoh/clauncher.git
   cd clauncher
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o clauncher ./cmd/clauncher/main.go
   ```

### Running the Application

The application automatically scans for locally installed LLM models at startup and seeds a model catalog from bundled data.

```bash
./clauncher
```

Press `r` to refresh the model list if you've downloaded new models.

## ⌨️ Controls

### Model Selection View

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate model list |
| `Enter` | Select model |
| `s` | Search HuggingFace |
| `d` | View model catalog |
| `m` | View benchmarks |
| `k` | Kill running servers |
| `r` | Refresh local models |
| `q` | Quit |

### Search View

| Key | Action |
|-----|--------|
| Type | Search query |
| `↑` / `↓` | Navigate results |
| `Enter` | Browse quant files |
| `b` / `Esc` | Back to selection |
| `Ctrl+C` | Quit |

### Quant Files View

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate quant files |
| `Enter` | Add to catalog |
| `b` / `Esc` | Back to search |

### Launch Options View

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate options |
| `Enter` | Launch selected option |
| `b` / `Esc` | Back to selection |

### Dashboard View

| Key | Action |
|-----|--------|
| `s` | Toggle Start/Stop |
| `b` / `Esc` | Back to selection |
| `q` | Quit |

## 🚀 Launch Modes

| Mode | Description |
|------|-------------|
| **Server** | Starts `llama-server` and shows the dashboard with live logs. |
| **CLI** | Runs `llama-cli` in a new terminal window. |
| **Claude Code** | Starts a llama server, configures `~/.claude/settings.json`, then launches `claude`. |
| **Opencode** | Starts a llama server, merges config into `~/.config/opencode/opencode.json`, then launches `opencode`. |
| **Crush** | Starts a llama server, merges config into `~/.config/crush/crush.json`, then launches `crush`. |

### AI Tool Configuration

- **Claude Code**: Sets `ANTHROPIC_BASE_URL`, auto-configures `CLAUDE_CODE_ATTRIBUTION_HEADER` for KV cache performance.
- **Opencode**: Merges `"llama-cpp"` provider into existing `opencode.json` with `http://localhost:<port>/v1` base URL.
- **Crush**: Merges `"llama-cpp"` provider into existing `crush.json` with `http://localhost:<port>/v1/` base URL. Warns if context < 4096.

## 🏗 Architecture

```
cmd/clauncher/main.go          — entry point: discovers models, wires runner + UI
pkg/model/model.go             — domain types (Model, LaunchOption, ProcessStatus)
pkg/server/command_runner.go   — real process lifecycle (start/stop/logs) + model discovery
pkg/server/runner_test.go      — tests against MockRunner
pkg/ui/model.go                — Bubble Tea App model (views, update, launch methods)
pkg/ui/messages/messages.go    — Bubble Tea message types
pkg/ui/theme/theme.go          — Lip Gloss color/style definitions
data/models.json               — bundled fallback catalog for seeding
```

### Control Flow

1. **Startup**: `main.go` calls `server.ListLocalModels()`, creates a `CommandRunner`, passes both to `ui.NewApp()`.
2. **Selection View**: User picks a model → transitions to **Launch Options View**.
3. **Launch Options**: User picks how to launch → `LaunchOptionSelectedMsg` is dispatched.
4. **Dispatch**:
   - `LaunchLlamaServer` → dashboard view + `startProcess()`
   - `LaunchLlamaCLI` → opens external terminal
   - `LaunchClaudeCode` / `LaunchOpencode` / `LaunchCrush` → background server + config + app launch
5. **Dashboard View**: Shows process status, streaming logs, and start/stop controls.

## 🛠 Development

### Building

```bash
go build -o clauncher ./cmd/clauncher/main.go
```

### Testing

The application uses a command-pattern for process execution. Swap the `CommandRunner` for the `MockRunner` to verify UI behavior without side effects.

```bash
go test ./...
```
