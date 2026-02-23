# Crates

Socratic coding tutor that runs in your terminal. The understanding layer for AI-assisted coding.

Claude Code writes your code. Crates makes sure you understand it. When AI-generated code touches concepts you haven't confirmed understanding of, Crates queues them as interactive lessons using Socratic dialogue, spaced repetition, and real code context.

## Install

**Homebrew:**

```bash
brew install ameal-dev/tap/crates
```

**Go:**

```bash
go install github.com/ameal-dev/crates/cmd/crates@latest
```

**From source:**

```bash
git clone https://github.com/ameal-dev/crates.git
cd crates
make build
# Binary at bin/crates
```

## Setup

Crates requires an Anthropic API key:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Then start the TUI:

```bash
crates
```

## Usage

```
crates           Start the TUI
crates --mcp     Start MCP server (for Claude Code integration)
crates --version Print version
crates export    Export all data as JSON
crates --help    Show help
```

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `t` | Browse topics |
| `r` | Review recall cards |
| `p` | View progress |
| `g` | Knowledge graph |
| `1-3` | Start queued lesson |
| `ctrl+h` | Go home |
| `ctrl+c` | Quit |

In a session:

| Key | Action |
|-----|--------|
| `enter` | Send answer |
| `ctrl+s` | Skip (ask for explanation) |
| `ctrl+d` | Complete session |
| `pgup/pgdn` | Scroll chat |
| `ctrl+h` | End session and go home |

## Claude Code Integration

Crates includes an MCP server that connects to Claude Code. When connected, Claude Code gains awareness of your knowledge gaps and adjusts its code generation accordingly.

Add to your Claude Code config (`~/.claude.json`):

```json
{
  "mcpServers": {
    "crates": {
      "command": "crates",
      "args": ["--mcp"]
    }
  }
}
```

The MCP server exposes two tools:

- **`crates_get_user_context`** -- Returns your weak topics so Claude Code can add explanatory comments where you need them.
- **`crates_record_exposure`** -- Records when AI-generated code touches concepts, feeding the lesson queue.

## How It Works

1. You use Claude Code to write code.
2. Crates detects concepts in the generated code via the MCP integration.
3. Concepts you haven't confirmed understanding of get queued as lessons.
4. When you open Crates, it teaches through Socratic dialogue -- asking questions, not lecturing.
5. Understanding checkpoints generate spaced repetition cards for long-term retention.
6. Your effective mastery feeds back to Claude Code, closing the loop.

## Data

All data is stored locally in `~/.crates/crates.db` (SQLite). Export your data anytime:

```bash
crates export > data.json
```

## License

MIT
