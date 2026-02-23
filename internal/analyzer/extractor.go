package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const maxDiffChars = 16000

// ClassifyDiff sends a diff to Claude for concept classification.
func ClassifyDiff(ctx context.Context, apiKey string, diff string, validSlugs []string) ([]ConceptMatch, error) {
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars]
	}

	slugList := strings.Join(validSlugs, ", ")
	prompt := fmt.Sprintf(`Analyze this code diff and identify which programming concepts are used.

Valid concept slugs: %s

For each concept you identify, return its slug, a confidence score (0.0-1.0), and a short code snippet (max 100 chars) showing where it appears.

Rules:
- Only return concepts from the valid slugs list above
- Only include concepts with confidence >= 0.7
- Max 5 concepts
- Only match concepts that are non-trivially present (actively used with meaningful logic, not just referenced)
- Do NOT match trivial patterns (bare imports, simple variable declarations)

Respond ONLY with a JSON array, no markdown or preamble:
[{"slug": "...", "confidence": 0.9, "snippet": "..."}]

If no concepts are detected, return an empty array: []

Code diff:
%s`, slugList, diff)

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("classify diff: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return parseClassificationResponse(text, validSlugs)
}

// parseClassificationResponse parses and filters the Claude classification response.
func parseClassificationResponse(text string, validSlugs []string) ([]ConceptMatch, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var matches []ConceptMatch
	if err := json.Unmarshal([]byte(text), &matches); err != nil {
		return nil, fmt.Errorf("parse classification JSON: %w (raw: %s)", err, text)
	}

	validSet := make(map[string]bool, len(validSlugs))
	for _, s := range validSlugs {
		validSet[s] = true
	}

	var filtered []ConceptMatch
	for _, m := range matches {
		if m.Confidence < 0.7 {
			continue
		}
		if !validSet[m.TopicSlug] {
			continue
		}
		if len(m.CodeSnippet) > 500 {
			m.CodeSnippet = m.CodeSnippet[:500]
		}
		filtered = append(filtered, m)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})
	if len(filtered) > 5 {
		filtered = filtered[:5]
	}

	return filtered, nil
}

// AnalyzeDiff is the main entry point: checks cache, runs fast path, optionally slow path.
func AnalyzeDiff(ctx context.Context, apiKey string, diff string, validSlugs []string, cache *Cache) ([]ConceptMatch, error) {
	if cache != nil {
		if cached := cache.Get(diff); cached != nil {
			return cached, nil
		}
	}

	// Fast path: regex patterns
	fastMatches := PatternMatch(diff)

	var allMatches []ConceptMatch
	allMatches = append(allMatches, fastMatches...)

	// Slow path: Claude classification if fast path found < 2 matches
	if len(fastMatches) < 2 && apiKey != "" {
		slowMatches, err := ClassifyDiff(ctx, apiKey, diff, validSlugs)
		if err == nil {
			allMatches = append(allMatches, slowMatches...)
		}
		// Non-fatal: if Claude call fails, we still have fast-path matches
	}

	// Deduplicate by slug, keeping higher confidence
	seen := make(map[string]ConceptMatch)
	for _, m := range allMatches {
		if existing, ok := seen[m.TopicSlug]; !ok || m.Confidence > existing.Confidence {
			seen[m.TopicSlug] = m
		}
	}

	result := make([]ConceptMatch, 0, len(seen))
	for _, m := range seen {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Confidence > result[j].Confidence
	})
	if len(result) > 5 {
		result = result[:5]
	}

	if cache != nil {
		cache.Set(diff, result)
	}

	return result, nil
}
