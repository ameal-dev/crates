package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/ameal-dev/crates/internal/db/models"
)

// RecallCardData represents a generated Q&A flashcard.
type RecallCardData struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// GenerateRecallCards creates Q&A flashcards from a list of checkpoints using a single Claude call.
func GenerateRecallCards(ctx context.Context, apiKey string, topicTitle string, checkpoints []models.Checkpoint) ([]RecallCardData, error) {
	if len(checkpoints) == 0 {
		return nil, nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	var cpList strings.Builder
	for i, cp := range checkpoints {
		cpList.WriteString(fmt.Sprintf("%d. Concept: %s — Evidence: %s\n", i+1, cp.Concept, cp.Evidence))
	}

	prompt := fmt.Sprintf(`Generate flashcards for spaced repetition review based on these knowledge checkpoints from a learning session about "%s":

%s

For each checkpoint, create 1-2 Q&A flashcard(s). Questions should test understanding, not just recall. Answers should be concise but complete.

Respond ONLY with JSON, no markdown or preamble. Use this exact format:
[{"question": "...", "answer": "..."}, ...]`, topicTitle, cpList.String())

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate recall cards: %w", err)
	}

	// Extract text from response
	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	// Strip markdown fences if present
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var cards []RecallCardData
	if err := json.Unmarshal([]byte(text), &cards); err != nil {
		return nil, fmt.Errorf("parse recall cards JSON: %w (raw: %s)", err, text)
	}

	return cards, nil
}
