package ai

import "testing"

func TestParseCheckpoints(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantClean      string
		wantCheckpoints int
		wantConcepts   []string
	}{
		{
			name:           "no checkpoints",
			input:          "Just a normal response with no markers.",
			wantClean:      "Just a normal response with no markers.",
			wantCheckpoints: 0,
		},
		{
			name:           "single checkpoint",
			input:          "Great job! You understand goroutines.\n<!--CHECKPOINT:goroutines|correctly explained lightweight threads-->",
			wantClean:      "Great job! You understand goroutines.",
			wantCheckpoints: 1,
			wantConcepts:   []string{"goroutines"},
		},
		{
			name:           "multiple checkpoints",
			input:          "You nailed both concepts!\n<!--CHECKPOINT:channels|understood buffered vs unbuffered-->\nLet's move on.\n<!--CHECKPOINT:select|explained non-blocking with default-->",
			wantClean:      "You nailed both concepts!\n\nLet's move on.",
			wantCheckpoints: 2,
			wantConcepts:   []string{"channels", "select"},
		},
		{
			name:           "checkpoint mid-text",
			input:          "Before <!--CHECKPOINT:maps|knows comma-ok pattern--> after",
			wantClean:      "Before  after",
			wantCheckpoints: 1,
			wantConcepts:   []string{"maps"},
		},
		{
			name:           "empty evidence",
			input:          "Good <!--CHECKPOINT:slices|-->",
			wantClean:      "Good",
			wantCheckpoints: 1,
			wantConcepts:   []string{"slices"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, checkpoints := ParseCheckpoints(tt.input)

			if cleaned != tt.wantClean {
				t.Errorf("cleaned:\ngot  %q\nwant %q", cleaned, tt.wantClean)
			}

			if len(checkpoints) != tt.wantCheckpoints {
				t.Fatalf("got %d checkpoints, want %d", len(checkpoints), tt.wantCheckpoints)
			}

			for i, want := range tt.wantConcepts {
				if checkpoints[i].Concept != want {
					t.Errorf("checkpoint[%d].Concept = %q, want %q", i, checkpoints[i].Concept, want)
				}
			}
		})
	}
}
