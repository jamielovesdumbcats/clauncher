# Clauncher

A powerful Terminal User Interface (TUI) designed to manage and launch your local LLM workflows, including Claude Code CLI and Llama CPP, with ease.

## ✨ Features

- **Model Management**: Effortlessly select and launch different local models.
- **Seamless TUI**: Built with the high-performance [Charmbracelet](https://charmbracelet.com/) ecosystem for a beautiful terminal experience.
- **Real-time Logs**: Monitor your model's output and server status directly within the interface.
- **Process Control**: Start, stop, and manage the lifecycle of your running models.
- **Developer Friendly**: Includes a built-in `MockRunner` for rapid UI development and testing.

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/) (1.21 or later recommended)

### Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
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

#### 🧪 Development / Test Mode (Recommended for testing UI)
To test the TUI logic without needing actual `llama` or `claude` processes running, use the built-in **Mock Mode**. This simulates process lifecycles and logs.

```bash
./clauncher
```
*Note: The current build defaults to `MockRunner` for safe development.*

#### 🛠 Production/Real Mode
To run with actual processes, ensure your command builder in `cmd/clauncher/main.go` points to your installed binaries.

```bash
./clauncher
```

## ⌨️ Controls

- `1`: Select the first model in the list.
- `q`: Quit the application.
- `s`: (Dashboard) Toggle Start/Stop for the selected model.

## 🏗 Architecture

The project follows a domain-driven design:

- `cmd/clauncher/`: Entry point.
- `pkg/model/`: Core domain entities.
- `pkg/server/`: Process orchestration and lifecycle management.
- `pkg/ui/`: TUI implementation using Bubble Tea and Lip Gloss.

## 🛠 Development

### Building
```bash
go build -o clauncher ./cmd/clauncher/main.go
```

### Testing
Since the application uses a command-pattern for process execution, you can easily swap the `CommandRunner` for the `MockRunner` in your tests to verify UI behavior without side effects.
