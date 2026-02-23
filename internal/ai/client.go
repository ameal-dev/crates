package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Streamer is the interface used by TUI screens for AI streaming.
type Streamer interface {
	StreamResponseWithChunks(ctx context.Context, systemPrompt string, messages []ChatMessage, ch chan<- tea.Msg) tea.Cmd
}

// Client wraps the Anthropic API client.
type Client struct {
	inner         anthropic.Client
	model         anthropic.Model
	fallbackModel anthropic.Model
}

// Verify Client satisfies Streamer at compile time.
var _ Streamer = (*Client)(nil)

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// NewClient creates a new AI client. The API key is read from ANTHROPIC_API_KEY env var.
func NewClient(apiKey string) *Client {
	inner := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		inner:         inner,
		model:         anthropic.ModelClaudeHaiku4_5,
		fallbackModel: anthropic.ModelClaudeSonnet4_5,
	}
}

// StreamResponse returns a tea.Cmd that streams an AI response.
// It accumulates all text and returns StreamDoneMsg with the full content.
// For chunk-by-chunk streaming, use StreamResponseWithChunks.
func (c *Client) StreamResponse(ctx context.Context, systemPrompt string, messages []ChatMessage) tea.Cmd {
	return func() tea.Msg {
		params := c.buildParams(systemPrompt, messages)
		content, err := c.doStream(ctx, params, nil)
		if err != nil && isOverloadedError(err) && content == "" {
			c.model = c.fallbackModel
			params = c.buildParamsWithModel(c.model, systemPrompt, messages)
			content, err = c.doStream(ctx, params, nil)
		}
		if err != nil {
			return StreamErrorMsg{Err: fmt.Errorf("stream error: %w", err)}
		}
		return StreamDoneMsg{FullContent: content}
	}
}

// StreamResponseWithChunks returns a tea.Cmd that streams chunks through a channel.
// Call this once, then use WaitForChunk to poll for individual chunks.
func (c *Client) StreamResponseWithChunks(ctx context.Context, systemPrompt string, messages []ChatMessage, ch chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ch <- StreamConnectingMsg{Model: string(c.model)}
		params := c.buildParams(systemPrompt, messages)
		content, err := c.doStream(ctx, params, ch)
		if err != nil && isOverloadedError(err) && content == "" {
			ch <- StreamFallbackMsg{
				FromModel: string(c.model),
				ToModel:   string(c.fallbackModel),
			}
			// Switch primary model so subsequent calls skip the dead model
			c.model = c.fallbackModel
			ch <- StreamConnectingMsg{Model: string(c.model)}
			params = c.buildParamsWithModel(c.model, systemPrompt, messages)
			content, err = c.doStream(ctx, params, ch)
		}
		if err != nil {
			ch <- StreamErrorMsg{Err: fmt.Errorf("stream error: %w", err)}
			return nil
		}
		ch <- StreamDoneMsg{FullContent: content}
		return nil
	}
}

// doStream runs the streaming loop, sending chunks to ch if non-nil.
// Returns the accumulated content and any stream error.
func (c *Client) doStream(ctx context.Context, params anthropic.MessageNewParams, ch chan<- tea.Msg) (string, error) {
	stream := c.inner.Messages.NewStreaming(ctx, params)

	var fullContent string
	for stream.Next() {
		event := stream.Current()
		switch ev := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := ev.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				if delta.Text != "" {
					fullContent += delta.Text
					if ch != nil {
						ch <- StreamChunkMsg{Content: delta.Text}
					}
				}
			}
		}
	}

	return fullContent, stream.Err()
}

// isOverloadedError checks whether an error is an Anthropic API overloaded or
// internal server error. Handles two forms:
//   - HTTP-level errors: *anthropic.Error with StatusCode 529 or 503
//   - Streaming errors: plain fmt.Errorf wrapping the SSE error event JSON,
//     containing "overloaded_error" or "api_error" in the message text
func isOverloadedError(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 529 || apiErr.StatusCode == 503
	}
	msg := err.Error()
	return strings.Contains(msg, `"overloaded_error"`) ||
		strings.Contains(msg, `"api_error"`)
}

// WaitForChunk returns a tea.Cmd that waits for the next message from the chunk channel.
func WaitForChunk(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return StreamDoneMsg{}
		}
		return msg
	}
}

func (c *Client) buildParamsWithModel(model anthropic.Model, systemPrompt string, messages []ChatMessage) anthropic.MessageNewParams {
	params := c.buildParams(systemPrompt, messages)
	params.Model = model
	return params
}

func (c *Client) buildParams(systemPrompt string, messages []ChatMessage) anthropic.MessageNewParams {
	var msgs []anthropic.MessageParam
	for _, m := range messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(
				anthropic.NewTextBlock(m.Content),
			))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(m.Content),
			))
		}
	}

	return anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: msgs,
	}
}
