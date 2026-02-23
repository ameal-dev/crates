package analyzer

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache()
	matches := []ConceptMatch{{TopicSlug: "go-goroutines", Confidence: 0.85, CodeSnippet: "go func()"}}
	c.Set("diff1", matches)

	got := c.Get("diff1")
	if got == nil {
		t.Fatal("expected cached result")
	}
	if len(got) != 1 {
		t.Fatalf("got %d matches, want 1", len(got))
	}
	if got[0].TopicSlug != "go-goroutines" {
		t.Errorf("slug = %q, want go-goroutines", got[0].TopicSlug)
	}
}

func TestCache_Miss(t *testing.T) {
	c := NewCache()
	got := c.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for cache miss, got %v", got)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := NewCache()
	matches := []ConceptMatch{{TopicSlug: "go-goroutines", Confidence: 0.85}}

	// Manually insert with old timestamp
	key := hashDiff("old-diff")
	c.mu.Lock()
	c.entries[key] = cacheEntry{matches: matches, createdAt: time.Now().Add(-25 * time.Hour)}
	c.mu.Unlock()

	got := c.Get("old-diff")
	if got != nil {
		t.Error("expected nil for expired entry")
	}
}

func TestCache_IsolatesMutations(t *testing.T) {
	c := NewCache()
	matches := []ConceptMatch{{TopicSlug: "go-goroutines", Confidence: 0.85}}
	c.Set("diff", matches)

	// Mutate original
	matches[0].TopicSlug = "mutated"

	got := c.Get("diff")
	if got[0].TopicSlug != "go-goroutines" {
		t.Error("cache should be isolated from mutations")
	}
}
