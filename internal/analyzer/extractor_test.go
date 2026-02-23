package analyzer

import (
	"testing"
)

func TestParseClassificationResponse_Valid(t *testing.T) {
	response := `[{"slug": "go-goroutines", "confidence": 0.92, "snippet": "go func() {...}"}]`
	validSlugs := []string{"go-goroutines", "go-channels"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if matches[0].TopicSlug != "go-goroutines" {
		t.Errorf("slug = %q, want go-goroutines", matches[0].TopicSlug)
	}
}

func TestParseClassificationResponse_StripsFences(t *testing.T) {
	response := "```json\n[{\"slug\": \"go-channels\", \"confidence\": 0.8, \"snippet\": \"make(chan int)\"}]\n```"
	validSlugs := []string{"go-channels"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
}

func TestParseClassificationResponse_FiltersLowConfidence(t *testing.T) {
	response := `[
		{"slug": "go-goroutines", "confidence": 0.92, "snippet": "..."},
		{"slug": "go-channels", "confidence": 0.5, "snippet": "..."}
	]`
	validSlugs := []string{"go-goroutines", "go-channels"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (low confidence filtered)", len(matches))
	}
}

func TestParseClassificationResponse_FiltersInvalidSlugs(t *testing.T) {
	response := `[
		{"slug": "go-goroutines", "confidence": 0.9, "snippet": "..."},
		{"slug": "unknown-topic", "confidence": 0.9, "snippet": "..."}
	]`
	validSlugs := []string{"go-goroutines"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (invalid slug filtered)", len(matches))
	}
}

func TestParseClassificationResponse_Max5(t *testing.T) {
	response := `[
		{"slug": "go-goroutines", "confidence": 0.95, "snippet": "..."},
		{"slug": "go-channels", "confidence": 0.94, "snippet": "..."},
		{"slug": "go-select", "confidence": 0.93, "snippet": "..."},
		{"slug": "go-context", "confidence": 0.92, "snippet": "..."},
		{"slug": "go-sync", "confidence": 0.91, "snippet": "..."},
		{"slug": "go-structs", "confidence": 0.90, "snippet": "..."},
		{"slug": "go-maps", "confidence": 0.89, "snippet": "..."}
	]`
	validSlugs := []string{"go-goroutines", "go-channels", "go-select", "go-context", "go-sync", "go-structs", "go-maps"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) > 5 {
		t.Errorf("got %d matches, want max 5", len(matches))
	}
}

func TestParseClassificationResponse_EmptyArray(t *testing.T) {
	response := `[]`
	matches, err := parseClassificationResponse(response, []string{"go-goroutines"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestParseClassificationResponse_TruncatesSnippet(t *testing.T) {
	longSnippet := make([]byte, 600)
	for i := range longSnippet {
		longSnippet[i] = 'x'
	}
	response := `[{"slug": "go-goroutines", "confidence": 0.9, "snippet": "` + string(longSnippet) + `"}]`
	validSlugs := []string{"go-goroutines"}
	matches, err := parseClassificationResponse(response, validSlugs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if len(matches[0].CodeSnippet) > 500 {
		t.Errorf("snippet length = %d, want <= 500", len(matches[0].CodeSnippet))
	}
}

func TestParseClassificationResponse_InvalidJSON(t *testing.T) {
	_, err := parseClassificationResponse("not json", []string{})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAnalyzeDiff_CacheHit(t *testing.T) {
	cache := NewCache()
	expected := []ConceptMatch{{TopicSlug: "go-goroutines", Confidence: 0.85}}
	cache.Set("some diff", expected)

	got, err := AnalyzeDiff(nil, "", "some diff", nil, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].TopicSlug != "go-goroutines" {
		t.Errorf("expected cache hit result, got %v", got)
	}
}

func TestAnalyzeDiff_FastPathOnly(t *testing.T) {
	diff := `
+go func() { work() }()
+sync.WaitGroup{}
+ch := make(chan int)
+select { case <-ch: }
`
	// With empty API key, slow path won't run
	got, err := AnalyzeDiff(nil, "", diff, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected fast-path matches")
	}
}

func TestDiffTruncation(t *testing.T) {
	// Verify that maxDiffChars constant is reasonable
	if maxDiffChars != 16000 {
		t.Errorf("maxDiffChars = %d, want 16000", maxDiffChars)
	}
}
