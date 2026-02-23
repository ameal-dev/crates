package mcp

import (
	"context"
	"database/sql"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server exposes Crates' knowledge graph to Claude Code via MCP.
type Server struct {
	db     *sql.DB
	apiKey string
	wg     sync.WaitGroup
	server *mcpsdk.Server
}

// NewServer creates and configures the MCP server with tools registered.
func NewServer(db *sql.DB, apiKey string) *Server {
	s := &Server{
		db:     db,
		apiKey: apiKey,
	}

	s.server = mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "crates",
		Version: "1.0.0",
	}, nil)

	s.registerTools()
	return s
}

// Run starts the MCP server on stdio transport. Blocks until context is cancelled or transport closes.
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcpsdk.StdioTransport{})
}

// Shutdown waits for all background goroutines to complete.
func (s *Server) Shutdown() {
	s.wg.Wait()
}
