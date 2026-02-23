package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ChatMessage represents a rendered chat message.
type ChatMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// ChatView is a scrollable chat viewport with role-based styling.
type ChatView struct {
	viewport viewport.Model
	messages []ChatMessage
	// Streaming state: partial content being accumulated
	streamingContent string
	topicSlug        string
	width, height    int
	renderer         *glamour.TermRenderer
}

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7aa2f7")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")).
			Bold(true)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true)

	messageBorderStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				MarginBottom(1)

	// Accent bars for message labels
	purpleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	blueAccent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	amberPrompt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Bold(true)
)

// NewChatView creates a new chat viewport.
func NewChatView(width, height int) ChatView {
	vp := viewport.New(width, height)
	vp.SetContent("")

	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("tokyo-night"),
		glamour.WithWordWrap(width-4),
	)

	return ChatView{
		viewport: vp,
		width:    width,
		height:   height,
		renderer: r,
	}
}

// SetSize updates the viewport dimensions.
func (c *ChatView) SetSize(width, height int) {
	if width == c.width && height == c.height {
		return
	}

	c.height = height
	c.viewport.Height = height

	// Only recreate renderer when width changes (word wrap depends on width)
	if width != c.width {
		c.width = width
		c.viewport.Width = width
		c.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("tokyo-night"),
			glamour.WithWordWrap(width-4),
		)
	}

	c.refreshContent()
}

// SetTopicSlug sets the topic slug used for term highlighting in code blocks.
func (c *ChatView) SetTopicSlug(slug string) {
	c.topicSlug = slug
}

// AddMessage appends a complete message to the chat.
func (c *ChatView) AddMessage(role, content string) {
	c.messages = append(c.messages, ChatMessage{Role: role, Content: content})
	c.streamingContent = ""
	c.refreshContent()
	c.viewport.GotoBottom()
}

// SetStreamingContent updates the partial streaming content for the current assistant response.
func (c *ChatView) SetStreamingContent(content string) {
	c.streamingContent = content
	c.refreshContent()
	c.viewport.GotoBottom()
}

// ClearStreaming clears the streaming buffer.
func (c *ChatView) ClearStreaming() {
	c.streamingContent = ""
}

// Update handles viewport scrolling messages.
func (c *ChatView) Update(msg interface{}) {
	// The viewport handles its own key/mouse input
	c.viewport, _ = c.viewport.Update(msg)
}

// View renders the chat viewport.
func (c ChatView) View() string {
	return c.viewport.View()
}

func (c *ChatView) refreshContent() {
	var parts []string

	for _, msg := range c.messages {
		parts = append(parts, c.renderMessage(msg.Role, msg.Content))
	}

	// Show streaming content as an in-progress assistant message
	if c.streamingContent != "" {
		parts = append(parts, c.renderMessage("assistant", c.streamingContent+"▊"))
	}

	content := strings.Join(parts, "\n")
	c.viewport.SetContent(content)
}

func (c *ChatView) renderMessage(role, content string) string {
	var label string
	switch role {
	case "user":
		label = blueAccent.Render("▎") + " " + userStyle.Render("You")
	case "assistant":
		label = purpleAccent.Render("▎") + " " + assistantStyle.Render("Crates")
	case "system":
		label = systemStyle.Render("System")
	default:
		label = role
	}

	rendered := content
	if role == "assistant" {
		rendered = c.renderAssistantContent(content)
	}

	return fmt.Sprintf("%s\n%s", label, messageBorderStyle.Render(rendered))
}

// renderAssistantContent handles code block extraction, Glamour prose rendering,
// and question detection for assistant messages.
func (c *ChatView) renderAssistantContent(content string) string {
	segments := ExtractSegments(content)
	terms := TopicTerms(c.topicSlug)

	var parts []string
	for _, seg := range segments {
		if seg.IsCode {
			parts = append(parts, RenderCodeBlock(seg.Content, seg.Language, c.width-4, terms))
		} else {
			if c.renderer != nil {
				if md, err := c.renderer.Render(seg.Content); err == nil {
					parts = append(parts, strings.TrimRight(md, "\n"))
				} else {
					parts = append(parts, seg.Content)
				}
			} else {
				parts = append(parts, seg.Content)
			}
		}
	}

	rendered := strings.Join(parts, "\n")

	// Question detection: mark the last question line with amber prompt
	lines := strings.Split(rendered, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimRight(lines[i], " \t")
		if strings.HasSuffix(trimmed, "?") {
			lines[i] = amberPrompt.Render("❯") + " " + lines[i]
			break
		}
	}

	return strings.Join(lines, "\n")
}

// MessageCount returns the number of messages in the chat.
func (c ChatView) MessageCount() int {
	return len(c.messages)
}
