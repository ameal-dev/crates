package screens

import (
	"database/sql"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/db"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/migrations"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.RunMigrations(database, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func seedTopic(t *testing.T, database *sql.DB) {
	t.Helper()
	err := queries.UpsertTopic(database, models.Topic{
		ID: "test-topic", Slug: "test-topic", Title: "Test Topic",
		Description: "A test topic", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
}

func TestSessionScreen_StreamChunkAccumulation(t *testing.T) {
	// Test that stream chunks accumulate correctly in the buffer.
	// This does NOT process StreamDoneMsg (which needs a DB).
	s := NewSessionScreen(nil, nil, "test-topic", "")
	s.SetSize(80, 24)
	s.chunkChan = make(chan tea.Msg, 64)
	s.streaming = true

	chunks := []string{"Hello ", "world, ", "how are ", "you?"}

	for i, chunk := range chunks {
		_, cmd := s.Update(ai.StreamChunkMsg{Content: chunk})

		// Each chunk should return a WaitForChunk command
		if cmd == nil {
			t.Fatalf("chunk %d: Update returned nil cmd, expected WaitForChunk", i)
		}

		expected := ""
		for j := 0; j <= i; j++ {
			expected += chunks[j]
		}
		if s.streamBuf != expected {
			t.Errorf("chunk %d: streamBuf = %q, want %q", i, s.streamBuf, expected)
		}
	}

	if !s.streaming {
		t.Error("should still be streaming after chunks (no done msg yet)")
	}
}

func TestSessionScreen_StreamDoneFlow(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database)

	s := NewSessionScreen(database, nil, "test-topic", "")
	s.SetSize(80, 24)
	s.chunkChan = make(chan tea.Msg, 64)
	s.streaming = true

	// Simulate session init (load topic + create session in DB)
	initCmd := s.initSession()
	initMsg := initCmd()
	s.Update(initMsg)

	// Accumulate some chunks
	s.Update(ai.StreamChunkMsg{Content: "Hello "})
	s.Update(ai.StreamChunkMsg{Content: "world"})

	// Process StreamDoneMsg (this writes to DB)
	_, cmd := s.Update(ai.StreamDoneMsg{FullContent: "Hello world"})

	if cmd != nil {
		t.Error("StreamDoneMsg should return nil cmd")
	}
	if s.streaming {
		t.Error("streaming should be false after StreamDoneMsg")
	}
	if s.streamBuf != "" {
		t.Errorf("streamBuf should be empty after done, got %q", s.streamBuf)
	}
	// Message should be saved
	if s.chatView.MessageCount() < 2 {
		t.Errorf("expected at least 2 messages (system + assistant), got %d", s.chatView.MessageCount())
	}
}

func TestSessionScreen_StreamErrorFlow(t *testing.T) {
	s := NewSessionScreen(nil, nil, "test-topic", "")
	s.SetSize(80, 24)
	s.chunkChan = make(chan tea.Msg, 64)
	s.streaming = true

	// Accumulate a partial chunk
	s.Update(ai.StreamChunkMsg{Content: "partial"})

	// Process error
	_, cmd := s.Update(ai.StreamErrorMsg{Err: errTest})

	if cmd != nil {
		t.Error("StreamErrorMsg should return nil cmd")
	}
	if s.streaming {
		t.Error("streaming should be false after error")
	}
	if s.streamBuf != "" {
		t.Errorf("streamBuf should be cleared after error, got %q", s.streamBuf)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestSessionScreen_ViewDuringStreaming_NoHang(t *testing.T) {
	s := NewSessionScreen(nil, nil, "test-topic", "")
	s.SetSize(80, 24)
	s.chunkChan = make(chan tea.Msg, 64)
	s.streaming = true

	// Simulate 20 streaming chunks with View() called after each.
	// This reproduces the exact condition that caused the hang:
	// View() calls chatView.SetSize() on every render, which used to
	// recreate the Glamour renderer with WithAutoStyle(), triggering
	// terminal probing that deadlocked with Bubble Tea's stdin reader.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			s.Update(ai.StreamChunkMsg{Content: "word "})
			_ = s.View() // This must not block
		}
	}()

	select {
	case <-done:
		// Success — no hang
	case <-time.After(5 * time.Second):
		t.Fatal("View() during streaming hung for >5s — likely terminal probing deadlock")
	}
}

func TestSessionScreen_ViewNoEscapeSequences(t *testing.T) {
	s := NewSessionScreen(nil, nil, "test-topic", "")
	s.SetSize(80, 24)

	view := s.View()

	// The view must NOT contain raw terminal escape sequences from color probing
	escapePatterns := []string{"rgb:", "11;rgb:", "1e1e/1e1e"}
	for _, pat := range escapePatterns {
		for i := 0; i <= len(view)-len(pat); i++ {
			if view[i:i+len(pat)] == pat {
				t.Errorf("View contains escape sequence %q — terminal probing leak into input", pat)
				break
			}
		}
	}
}

func TestSessionScreen_RepeatedViewDoesNotDoubleRender(t *testing.T) {
	s := NewSessionScreen(nil, nil, "test-topic", "")
	s.SetSize(80, 24)
	s.streaming = true
	s.chunkChan = make(chan tea.Msg, 64)

	// Simulate streaming with View() called after each chunk.
	// Before the fix, each View() recreated the Glamour renderer and
	// called refreshContent(), doubling the work per chunk.
	start := time.Now()
	for i := 0; i < 50; i++ {
		s.Update(ai.StreamChunkMsg{Content: "chunk "})
		_ = s.View()
	}
	elapsed := time.Since(start)

	// With the SetSize guard, 50 iterations should complete well under 2s.
	// Without it, Glamour renderer recreation on every View() makes this slow.
	if elapsed > 2*time.Second {
		t.Errorf("50 stream+view cycles took %v — likely double-rendering on each View()", elapsed)
	}
}
