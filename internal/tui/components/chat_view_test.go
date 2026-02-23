package components

import (
	"strings"
	"testing"
)

func TestNewChatView(t *testing.T) {
	cv := NewChatView(80, 20)
	if cv.width != 80 {
		t.Errorf("width = %d, want 80", cv.width)
	}
	if cv.height != 20 {
		t.Errorf("height = %d, want 20", cv.height)
	}
	if cv.renderer == nil {
		t.Fatal("renderer is nil after NewChatView")
	}
	if cv.MessageCount() != 0 {
		t.Errorf("MessageCount = %d, want 0", cv.MessageCount())
	}
}

func TestSetSize_RecreatesRendererOnWidthChange(t *testing.T) {
	cv := NewChatView(80, 20)
	origRenderer := cv.renderer

	// Changing width should recreate the renderer (new word wrap width)
	cv.SetSize(120, 20)
	if cv.renderer == origRenderer {
		t.Error("renderer was NOT recreated after width change — word wrap would be wrong")
	}
	if cv.width != 120 {
		t.Errorf("width = %d, want 120", cv.width)
	}
}

func TestSetSize_SkipsRecreateOnSameDimensions(t *testing.T) {
	cv := NewChatView(80, 20)
	origRenderer := cv.renderer

	// Same dimensions should NOT recreate the renderer.
	// This is critical: View() calls SetSize on every render, including
	// during streaming. Recreating the renderer each time is wasteful and
	// was the root cause of the terminal-probing deadlock with WithAutoStyle().
	cv.SetSize(80, 20)
	if cv.renderer != origRenderer {
		t.Error("renderer was recreated for identical dimensions — causes double-render during streaming")
	}
}

func TestSetSize_HeightOnlyChange(t *testing.T) {
	cv := NewChatView(80, 20)
	origRenderer := cv.renderer

	// Height-only change: viewport height needs updating, but renderer
	// (which depends on width for word wrap) should be reused.
	cv.SetSize(80, 30)
	if cv.renderer != origRenderer {
		t.Error("renderer was recreated for height-only change — wasteful, only width affects word wrap")
	}
	if cv.height != 30 {
		t.Errorf("height = %d, want 30", cv.height)
	}
}

func TestStreamingContent_NoEscapeSequences(t *testing.T) {
	cv := NewChatView(80, 20)

	// Simulate a streaming session: add a system message, then stream chunks
	cv.AddMessage("system", "Topic: Go basics")
	cv.SetStreamingContent("Hello, let me help you learn")
	cv.SetStreamingContent("Hello, let me help you learn about Go.")

	view := cv.View()

	// The rendered view must NOT contain raw terminal escape sequences.
	// These patterns appear when glamour.WithAutoStyle() probes the terminal.
	escapePatterns := []string{
		"rgb:",
		"\\033]",
		"11;rgb:",
		"1e1e/1e1e/2e2e",
	}
	for _, pat := range escapePatterns {
		if strings.Contains(view, pat) {
			t.Errorf("view contains escape sequence pattern %q — terminal probing leak", pat)
		}
	}
}

func TestStreamingContent_RepeatedSetSize(t *testing.T) {
	cv := NewChatView(80, 20)
	cv.AddMessage("assistant", "First message")

	// Simulate what happens during streaming: View() calls SetSize every render,
	// and SetStreamingContent is called on every chunk. This used to cause
	// double-rendering (refreshContent called twice per chunk).
	for i := 0; i < 50; i++ {
		cv.SetStreamingContent(strings.Repeat("chunk ", i+1))
		// View() calls SetSize with same dimensions on every render
		cv.SetSize(80, 20)
	}

	// Should complete without hanging. The test itself passing is the assertion.
	if cv.MessageCount() != 1 {
		t.Errorf("MessageCount = %d, want 1 (streaming content is not a committed message)", cv.MessageCount())
	}
}

func TestAddMessage_ClearsStreamingContent(t *testing.T) {
	cv := NewChatView(80, 20)
	cv.SetStreamingContent("partial streaming response")
	cv.AddMessage("assistant", "full response")

	if cv.streamingContent != "" {
		t.Error("streamingContent not cleared after AddMessage")
	}
	if cv.MessageCount() != 1 {
		t.Errorf("MessageCount = %d, want 1", cv.MessageCount())
	}
}

func TestClearStreaming(t *testing.T) {
	cv := NewChatView(80, 20)
	cv.SetStreamingContent("partial")
	cv.ClearStreaming()

	if cv.streamingContent != "" {
		t.Error("streamingContent not cleared after ClearStreaming")
	}
}
