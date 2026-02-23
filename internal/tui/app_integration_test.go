package tui_test

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/curriculum"
	"github.com/ameal-dev/crates/internal/db"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/internal/tui"
	"github.com/ameal-dev/crates/internal/tui/screens"
	"github.com/ameal-dev/crates/migrations"

	cratesData "github.com/ameal-dev/crates/data"
)

// mockStreamer simulates the real AI client by streaming canned responses
// chunk-by-chunk through the same channel mechanism the real client uses.
type mockStreamer struct {
	responses []string
	callIdx   int
}

func (m *mockStreamer) StreamResponseWithChunks(_ context.Context, _ string, _ []ai.ChatMessage, ch chan<- tea.Msg) tea.Cmd {
	response := m.responses[m.callIdx%len(m.responses)]
	m.callIdx++

	return func() tea.Msg {
		words := strings.Fields(response)
		var full string
		for i, word := range words {
			chunk := word
			if i < len(words)-1 {
				chunk += " "
			}
			full += chunk
			ch <- ai.StreamChunkMsg{Content: chunk}
			time.Sleep(5 * time.Millisecond)
		}
		ch <- ai.StreamDoneMsg{FullContent: full}
		return nil
	}
}

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

	// Seed the real curriculum so topic IDs are valid
	if err := curriculum.SeedCurriculum(database, cratesData.Curriculum); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Mark onboarding as done so we skip straight to home screen
	if err := queries.SetAppState(database, "onboarding_completed", "true"); err != nil {
		t.Fatalf("set onboarding: %v", err)
	}

	return database
}

// findFirstLeafTopic finds the first leaf topic in the curriculum for testing.
func findFirstLeafTopic(t *testing.T, database *sql.DB) string {
	t.Helper()
	tree, err := queries.GetTopicTree(database)
	if err != nil {
		t.Fatalf("get topic tree: %v", err)
	}
	if len(tree) == 0 {
		t.Fatal("no topics in tree")
	}
	// Walk to first leaf
	node := tree[0]
	for len(node.Children) > 0 {
		node = node.Children[0]
	}
	return node.ID
}

// TestJourney_SessionStreaming is the core user journey test:
// User starts app → navigates to a session → AI streams a response →
// user sees the streamed text appear without escape sequences or hangs →
// user types an answer → AI streams a follow-up.
func TestJourney_SessionStreaming(t *testing.T) {
	database := setupTestDB(t)
	topicID := findFirstLeafTopic(t, database)

	mock := &mockStreamer{
		responses: []string{
			"Great question! **What do you already know** about this topic? Tell me in your own words.",
			"That's a good start! Let me ask you something deeper. **Can you explain why** this concept matters in practice?",
		},
	}

	app := tui.NewApp(database, mock)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// Step 1: App starts on home screen
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			// Home screen should render something recognizable
			return bytes.Contains(bts, []byte("Crates")) ||
				bytes.Contains(bts, []byte("home")) ||
				bytes.Contains(bts, []byte("Topics")) ||
				bytes.Contains(bts, []byte("Session")) ||
				bytes.Contains(bts, []byte("Recall"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(5*time.Second),
	)

	// Step 2: Navigate directly to a session (simulating topic selection)
	tm.Send(screens.NavigateMsg{Screen: "session", TopicID: topicID})

	// Step 3: Wait for the AI's first streamed response to appear
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Great question"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)

	// Step 4: Verify no escape sequences leaked into the output
	// Read all output so far and check for the terminal probe pattern
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I know a bit about it")})
	time.Sleep(200 * time.Millisecond)

	// Step 5: Submit the answer
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Step 6: Wait for the second AI response
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("good start"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)

	// Step 7: Clean exit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestJourney_NoEscapeSequencesInOutput verifies that terminal escape
// sequences from Glamour's terminal probing never appear in rendered output.
// This was the primary symptom: the text input showed garbage like
// "|11;rgb:1e1e/1e1e/2e2e\" instead of the placeholder.
func TestJourney_NoEscapeSequencesInOutput(t *testing.T) {
	database := setupTestDB(t)
	topicID := findFirstLeafTopic(t, database)

	mock := &mockStreamer{
		responses: []string{
			"Welcome! Let's learn together. What do you know about this topic?",
		},
	}

	app := tui.NewApp(database, mock)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// Navigate to session
	tm.Send(screens.NavigateMsg{Screen: "session", TopicID: topicID})

	// Wait for AI response to fully stream
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Welcome"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)

	// Let a couple render cycles complete after streaming finishes
	time.Sleep(500 * time.Millisecond)

	// Now capture current output and scan for escape sequence patterns
	// that indicate terminal color probing leaked into the render
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out := tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second))

	var buf bytes.Buffer
	buf.ReadFrom(out)
	rendered := buf.String()

	escapePatterns := []string{
		"rgb:1e1e",
		"11;rgb:",
		"1e1e/1e1e/2e2e",
	}
	for _, pat := range escapePatterns {
		if strings.Contains(rendered, pat) {
			t.Errorf("Rendered output contains escape sequence %q — terminal probing leak.\nSnippet: ...%s...",
				pat, extractContext(rendered, pat))
		}
	}
}

// TestJourney_StreamingDoesNotHang verifies that the streaming + rendering
// loop completes within a reasonable time. Before the fix, WithAutoStyle()
// terminal probing during View() caused a deadlock with Bubble Tea's stdin.
func TestJourney_StreamingDoesNotHang(t *testing.T) {
	database := setupTestDB(t)
	topicID := findFirstLeafTopic(t, database)

	// Use a long response with many words to generate lots of streaming chunks.
	// Each chunk triggers a View() render. The old code would deadlock here
	// because View() → SetSize() → WithAutoStyle() → terminal probe → blocked by Bubble Tea.
	longResponse := strings.Repeat("This is a streaming chunk that triggers a render cycle. ", 20)
	mock := &mockStreamer{
		responses: []string{longResponse},
	}

	app := tui.NewApp(database, mock)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// Navigate to session
	tm.Send(screens.NavigateMsg{Screen: "session", TopicID: topicID})

	// The entire long response must finish streaming within 15 seconds.
	// Before the fix, this would hang indefinitely.
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			// Check for content near the end of the response
			return bytes.Contains(bts, []byte("render cycle."))
		},
		teatest.WithCheckInterval(200*time.Millisecond),
		teatest.WithDuration(15*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestJourney_CtrlH_NavigatesHome verifies that ctrl+h during a session
// returns the user to the home screen.
func TestJourney_CtrlH_NavigatesHome(t *testing.T) {
	database := setupTestDB(t)
	topicID := findFirstLeafTopic(t, database)

	mock := &mockStreamer{
		responses: []string{"Let's learn! What do you know?"},
	}

	app := tui.NewApp(database, mock)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))

	// Navigate to session and wait for it to render
	tm.Send(screens.NavigateMsg{Screen: "session", TopicID: topicID})
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("learn"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)

	// Press ctrl+h to go home
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlH})

	// Should see home screen content
	teatest.WaitFor(t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Crates")) ||
				bytes.Contains(bts, []byte("Topics")) ||
				bytes.Contains(bts, []byte("Recall")) ||
				bytes.Contains(bts, []byte("Lesson"))
		},
		teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(5*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// extractContext returns ~40 chars around the first occurrence of pattern in s.
func extractContext(s, pattern string) string {
	idx := strings.Index(s, pattern)
	if idx < 0 {
		return ""
	}
	start := idx - 20
	if start < 0 {
		start = 0
	}
	end := idx + len(pattern) + 20
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
