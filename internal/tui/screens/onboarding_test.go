package screens

import (
	"testing"

	"github.com/ameal-dev/crates/internal/db/models"
)

func TestFirstLeafTopic(t *testing.T) {
	// Single leaf node
	leaf := models.Topic{ID: "go-variables"}
	if got := firstLeafTopic(leaf); got != "go-variables" {
		t.Errorf("got %q, want %q", got, "go-variables")
	}

	// One level deep
	parent := models.Topic{
		ID: "go-basics",
		Children: []models.Topic{
			{ID: "go-variables"},
			{ID: "go-control-flow"},
		},
	}
	if got := firstLeafTopic(parent); got != "go-variables" {
		t.Errorf("got %q, want %q", got, "go-variables")
	}

	// Two levels deep
	root := models.Topic{
		ID: "go",
		Children: []models.Topic{
			{
				ID: "go-basics",
				Children: []models.Topic{
					{ID: "go-variables"},
					{ID: "go-control-flow"},
				},
			},
			{
				ID: "go-concurrency",
				Children: []models.Topic{
					{ID: "go-goroutines"},
				},
			},
		},
	}
	if got := firstLeafTopic(root); got != "go-variables" {
		t.Errorf("got %q, want %q", got, "go-variables")
	}
}
