package analyzer

import (
	"testing"

	"github.com/ameal-dev/crates/data"
)

func TestLeafSlugs(t *testing.T) {
	slugs := LeafSlugs(data.Curriculum)

	if len(slugs) == 0 {
		t.Fatal("expected non-empty slugs")
	}

	// Should have leaf topics, not parent categories
	slugSet := make(map[string]bool)
	for _, s := range slugs {
		slugSet[s] = true
	}

	// Leaves should be present
	expectedLeaves := []string{"go-goroutines", "go-channels", "react-effects", "react-state", "js-promises"}
	for _, leaf := range expectedLeaves {
		if !slugSet[leaf] {
			t.Errorf("expected leaf slug %q to be present", leaf)
		}
	}

	// Parent categories should NOT be present
	parentSlugs := []string{"go", "go-concurrency", "javascript", "react", "typescript"}
	for _, parent := range parentSlugs {
		if slugSet[parent] {
			t.Errorf("parent slug %q should not be in leaf slugs", parent)
		}
	}
}
