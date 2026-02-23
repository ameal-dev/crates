package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ameal-dev/crates/data"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/curriculum"
	"github.com/ameal-dev/crates/internal/db"
	cratesexport "github.com/ameal-dev/crates/internal/export"
	cratesmcp "github.com/ameal-dev/crates/internal/mcp"
	"github.com/ameal-dev/crates/internal/tui"
	"github.com/ameal-dev/crates/migrations"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-v":
			fmt.Println("crates " + version)
			return
		case "--help", "-h":
			printUsage()
			return
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`crates - Socratic coding tutor

Usage:
  crates           Start the TUI
  crates --mcp     Start MCP server (for Claude Code integration)
  crates --version Print version
  crates export    Export all data as JSON to stdout

Environment:
  ANTHROPIC_API_KEY  Required. Your Anthropic API key.

MCP Integration:
  Add to your Claude Code config (~/.claude.json):
  {
    "mcpServers": {
      "crates": {
        "command": "crates",
        "args": ["--mcp"]
      }
    }
  }
`)
}

func run() error {
	// Check for subcommand/flags
	mcpMode := false
	exportMode := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--mcp":
			mcpMode = true
		case "export":
			exportMode = true
		}
	}

	// Determine DB path
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	dbDir := filepath.Join(home, ".crates")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	dbPath := filepath.Join(dbDir, "crates.db")

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	// Run migrations
	if err := db.RunMigrations(database, migrations.FS, "."); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Seed curriculum
	if err := curriculum.SeedCurriculum(database, data.Curriculum); err != nil {
		return fmt.Errorf("seed curriculum: %w", err)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	if mcpMode {
		return runMCP(database, apiKey)
	}

	if exportMode {
		return cratesexport.Export(database, os.Stdout)
	}

	return runTUI(database, apiKey)
}

func runMCP(database *sql.DB, apiKey string) error {
	// Redirect all logging away from stdout — MCP uses stdio for JSON-RPC
	log.SetOutput(io.Discard)

	server := cratesmcp.NewServer(database, apiKey)
	defer server.Shutdown()

	return server.Run(context.Background())
}

func runTUI(database *sql.DB, apiKey string) (err error) {
	// Recover from panics to restore terminal state
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	var aiClient ai.Streamer
	if apiKey != "" {
		aiClient = ai.NewClient(apiKey)
	}

	app := tui.NewApp(database, aiClient, apiKey)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
