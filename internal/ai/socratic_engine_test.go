package ai

import (
	"strings"
	"testing"

	"github.com/ameal-dev/crates/internal/db/models"
)

func TestBuildSystemPrompt(t *testing.T) {
	topic := models.Topic{
		ID:          "go-goroutines",
		Title:       "Goroutines",
		Description: "Launching goroutines, lifecycle, WaitGroup",
	}

	tests := []struct {
		name     string
		progress models.TopicProgress
		wantContains []string
	}{
		{
			name:     "new student",
			progress: models.TopicProgress{MasteryLevel: 0},
			wantContains: []string{
				"Socratic coding tutor",
				"Goroutines",
				"Mastery level: 0/5",
				"hint ladder",
				"CHECKPOINT",
				"new to this topic",
			},
		},
		{
			name:     "beginner",
			progress: models.TopicProgress{MasteryLevel: 1, HintsUsed: 3},
			wantContains: []string{
				"Mastery level: 1/5",
				"Hints used: 3",
				"beginner understanding",
			},
		},
		{
			name:     "intermediate",
			progress: models.TopicProgress{MasteryLevel: 3},
			wantContains: []string{
				"intermediate understanding",
				"edge cases",
			},
		},
		{
			name:     "advanced",
			progress: models.TopicProgress{MasteryLevel: 5},
			wantContains: []string{
				"advanced understanding",
				"nuances",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildSystemPrompt(topic, tt.progress, nil)

			for _, want := range tt.wantContains {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt missing %q", want)
				}
			}
		})
	}
}

func TestBuildSystemPrompt_WithExposures(t *testing.T) {
	topic := models.Topic{
		ID:          "go-interfaces",
		Title:       "Interface Basics",
		Description: "Defining and implementing interfaces in Go",
	}

	snippet := `type Streamer interface { StreamResponse(ctx context.Context) }`

	exposures := []models.ConceptExposure{
		{
			ID:          "exp-1",
			TopicID:     "go-interfaces",
			Source:      "AI_ACCEPTED",
			CodeSnippet: &snippet,
			Confidence:  0.92,
		},
	}

	t.Run("includes code context section", func(t *testing.T) {
		progress := models.TopicProgress{MasteryLevel: 0}
		prompt := BuildSystemPrompt(topic, progress, exposures)

		wantContains := []string{
			"Code Context",
			"Streamer interface",
			"AI_ACCEPTED",
			"Start by showing the student",
			"seen this concept in code",
		}
		for _, want := range wantContains {
			if !strings.Contains(prompt, want) {
				t.Errorf("prompt missing %q", want)
			}
		}

		// Should NOT contain the generic "new to this topic" phrasing
		if strings.Contains(prompt, "new to this topic") {
			t.Error("prompt should not say 'new to this topic' when exposures exist")
		}
	})

	t.Run("skips empty snippets", func(t *testing.T) {
		empty := ""
		exposuresWithEmpty := []models.ConceptExposure{
			{ID: "exp-2", TopicID: "go-interfaces", Source: "AI_ACCEPTED", CodeSnippet: &empty},
			{ID: "exp-3", TopicID: "go-interfaces", Source: "AI_ACCEPTED", CodeSnippet: nil},
		}
		progress := models.TopicProgress{MasteryLevel: 1}
		prompt := BuildSystemPrompt(topic, progress, exposuresWithEmpty)

		// Code Context section should still appear (exposures slice is non-empty)
		if !strings.Contains(prompt, "Code Context") {
			t.Error("prompt should contain Code Context header")
		}
		// But no "Source:" lines since both snippets are empty/nil
		if strings.Contains(prompt, "Source:") {
			t.Error("prompt should not contain Source lines for empty snippets")
		}
	})
}
