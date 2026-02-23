package screens

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

func TestKnowledgeGraphScreen_EmptyState(t *testing.T) {
	database := setupTestDB(t)

	g := NewKnowledgeGraphScreen(database)
	g.SetSize(120, 40)

	// Run the init command
	cmd := g.Init()
	if cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
	msg := cmd()
	g.Update(msg)

	view := g.View()
	if !strings.Contains(view, "Knowledge Graph") {
		t.Error("View should contain title")
	}
	if !strings.Contains(view, "Start a session") {
		t.Errorf("Empty state should show guidance message, got: %s", view)
	}
}

func TestKnowledgeGraphScreen_LoadsData(t *testing.T) {
	database := setupTestDB(t)

	// Seed a 3-level topic tree
	goRoot := models.Topic{ID: "go", Slug: "go", Title: "Go", SortOrder: 1}
	goBasics := models.Topic{ID: "go-basics", ParentID: &goRoot.ID, Slug: "go/basics", Title: "Basics", SortOrder: 1}
	goVars := models.Topic{ID: "go-vars", ParentID: &goBasics.ID, Slug: "go/basics/variables", Title: "Variables", SortOrder: 1}
	goFuncs := models.Topic{ID: "go-funcs", ParentID: &goBasics.ID, Slug: "go/basics/functions", Title: "Functions", SortOrder: 2}

	for _, topic := range []models.Topic{goRoot, goBasics, goVars, goFuncs} {
		if err := queries.UpsertTopic(database, topic); err != nil {
			t.Fatalf("seed topic: %v", err)
		}
	}

	// Add progress for one topic
	now := time.Now().UTC()
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID:            "go-vars",
		MasteryLevel:       3,
		ConfidenceModifier: 0.9,
		AIExposureCount:    2,
		AuthoredCount:      5,
		LastConfirmedAt:    &now,
	}); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	g := NewKnowledgeGraphScreen(database)
	g.SetSize(120, 40)

	msg := g.Init()()
	g.Update(msg)

	view := g.View()
	if !strings.Contains(view, "Go") {
		t.Error("View should contain language name 'Go'")
	}
	if !strings.Contains(view, "Basics") {
		t.Error("View should contain category name 'Basics'")
	}
	if len(g.flatCells) != 2 {
		t.Errorf("expected 2 flat cells (Variables, Functions), got %d", len(g.flatCells))
	}
}

func TestKnowledgeGraphScreen_CellRendering(t *testing.T) {
	tests := []struct {
		name         string
		node         graphTopicNode
		wantText     string
		wantDiag     string
	}{
		{
			name:     "solid mastery",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 4, confidenceMod: 1.0, effectiveMastery: 4.0},
			wantText: cellSolid,
		},
		{
			name:     "good mastery",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 3, confidenceMod: 0.9, effectiveMastery: 2.7},
			wantText: cellGood,
		},
		{
			name:     "shaky mastery",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 2, confidenceMod: 0.8, effectiveMastery: 1.6},
			wantText: cellShaky,
		},
		{
			name:     "weak mastery",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 1, confidenceMod: 0.5, effectiveMastery: 0.5},
			wantText: cellWeak,
		},
		{
			name:     "untouched",
			node:     graphTopicNode{hasProgress: false},
			wantText: cellUntouched,
		},
		{
			name:     "amber variant - high mastery low confidence",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 4, confidenceMod: 0.5, effectiveMastery: 2.0},
			wantText: cellShaky, // renders as shaky despite high mastery
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cellText(tt.node)
			if got != tt.wantText {
				t.Errorf("cellText() = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestKnowledgeGraphScreen_AmberColor(t *testing.T) {
	// High mastery + low confidence should produce amber color, not green/teal
	node := graphTopicNode{hasProgress: true, masteryLevel: 4, confidenceMod: 0.5, effectiveMastery: 2.0}
	got := cellColor(node)
	if got != colorAmber {
		t.Errorf("cellColor() = %v, want amber (%v)", got, colorAmber)
	}
}

func TestKnowledgeGraphScreen_Navigation(t *testing.T) {
	g := &KnowledgeGraphScreen{
		flatCells: []graphTopicNode{
			{topicID: "a", title: "A"},
			{topicID: "b", title: "B"},
			{topicID: "c", title: "C"},
		},
		selectedIdx: 0,
		width:       120,
		height:      40,
	}

	// Move down
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.selectedIdx != 1 {
		t.Errorf("after j: selectedIdx = %d, want 1", g.selectedIdx)
	}

	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.selectedIdx != 2 {
		t.Errorf("after j: selectedIdx = %d, want 2", g.selectedIdx)
	}

	// Can't go past end
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.selectedIdx != 2 {
		t.Errorf("after j at end: selectedIdx = %d, want 2", g.selectedIdx)
	}

	// Move up
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if g.selectedIdx != 1 {
		t.Errorf("after k: selectedIdx = %d, want 1", g.selectedIdx)
	}

	// Can't go past beginning
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if g.selectedIdx != 0 {
		t.Errorf("after k at start: selectedIdx = %d, want 0", g.selectedIdx)
	}
}

func TestKnowledgeGraphScreen_EnterNavigatesToSession(t *testing.T) {
	g := &KnowledgeGraphScreen{
		flatCells: []graphTopicNode{
			{topicID: "test-topic", title: "Test"},
		},
		selectedIdx: 0,
		width:       120,
		height:      40,
	}

	_, cmd := g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return a navigate command")
	}

	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Screen != "session" {
		t.Errorf("navigate screen = %q, want session", nav.Screen)
	}
	if nav.TopicID != "test-topic" {
		t.Errorf("navigate topicID = %q, want test-topic", nav.TopicID)
	}
}

func TestKnowledgeGraphScreen_ConfidenceDiagnostic(t *testing.T) {
	twoWeeksAgo := time.Now().Add(-14 * 24 * time.Hour)
	sixtyDaysAgo := time.Now().Add(-60 * 24 * time.Hour)

	tests := []struct {
		name     string
		node     graphTopicNode
		contains string
	}{
		{
			name:     "solid understanding",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 4, confidenceMod: 0.95, effectiveMastery: 3.8},
			contains: "Solid",
		},
		{
			name:     "AI erosion",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 3, confidenceMod: 0.4, aiExposureCount: 10, authoredCount: 1, effectiveMastery: 1.2},
			contains: "AI exposure",
		},
		{
			name:     "time decay",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 2, confidenceMod: 0.6, lastConfirmedAt: &sixtyDaysAgo, effectiveMastery: 1.2},
			contains: "last confirmed",
		},
		{
			name:     "high mastery low confidence",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 4, confidenceMod: 0.6, lastConfirmedAt: &twoWeeksAgo, effectiveMastery: 2.4},
			contains: "Consider a session",
		},
		{
			name:     "needs study",
			node:     graphTopicNode{hasProgress: true, masteryLevel: 1, confidenceMod: 0.5, effectiveMastery: 0.5},
			contains: "Needs focused study",
		},
		{
			name:     "no progress",
			node:     graphTopicNode{hasProgress: false},
			contains: "Needs focused study",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := confidenceDiagnostic(tt.node)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("diagnostic = %q, want to contain %q", got, tt.contains)
			}
		})
	}
}

func TestKnowledgeGraphScreen_ResponsiveLayout(t *testing.T) {
	database := setupTestDB(t)

	// Seed minimal tree
	goRoot := models.Topic{ID: "go", Slug: "go", Title: "Go", SortOrder: 1}
	goBasics := models.Topic{ID: "go-basics", ParentID: &goRoot.ID, Slug: "go/basics", Title: "Basics", SortOrder: 1}
	goVars := models.Topic{ID: "go-vars", ParentID: &goBasics.ID, Slug: "go/basics/variables", Title: "Variables", SortOrder: 1}

	for _, topic := range []models.Topic{goRoot, goBasics, goVars} {
		if err := queries.UpsertTopic(database, topic); err != nil {
			t.Fatalf("seed topic: %v", err)
		}
	}

	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-vars", MasteryLevel: 2, ConfidenceModifier: 0.8,
	}); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	// Wide layout should contain sidebar border character
	g := NewKnowledgeGraphScreen(database)
	g.SetSize(120, 40)
	msg := g.Init()()
	g.Update(msg)

	wideView := g.View()

	// Narrow layout should NOT have sidebar border, should have compact detail
	g.SetSize(80, 40)
	narrowView := g.View()

	// Both views should render without panics and contain the title
	if !strings.Contains(wideView, "Knowledge Graph") {
		t.Error("wide view missing title")
	}
	if !strings.Contains(narrowView, "Knowledge Graph") {
		t.Error("narrow view missing title")
	}

	// Wide view should have the sidebar with mastery info
	if !strings.Contains(wideView, "Mastery:") {
		t.Error("wide view should contain sidebar with Mastery:")
	}

	// Narrow view should have compact detail with mastery info
	if !strings.Contains(narrowView, "mastery:") {
		t.Error("narrow view should contain compact detail")
	}
}
