package queue

import (
	"database/sql"
	"testing"
	"time"

	"github.com/ameal-dev/crates/internal/db"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/migrations"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.RunMigrations(database, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func seedTopic(t *testing.T, database *sql.DB, id, slug, title string) {
	t.Helper()
	err := queries.UpsertTopic(database, models.Topic{
		ID: id, Slug: slug, Title: title, Description: "test", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
}

func TestBuildDailyQueue_Empty(t *testing.T) {
	database := setupTestDB(t)
	items, err := BuildDailyQueue(database, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestBuildDailyQueue_ExcludesHighMastery(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")

	// effective_mastery = 4.0 * 1.0 = 4.0 > 3.0 → excluded
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-goroutines", MasteryLevel: 4, ConfidenceModifier: 1.0,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	items, err := BuildDailyQueue(database, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (high mastery excluded)", len(items))
	}
}

func TestBuildDailyQueue_Max3(t *testing.T) {
	database := setupTestDB(t)
	for i := 0; i < 5; i++ {
		id := "topic-" + string(rune('a'+i))
		seedTopic(t, database, id, id, "Topic "+string(rune('A'+i)))
		_ = queries.UpsertTopicProgress(database, models.TopicProgress{
			TopicID: id, MasteryLevel: 1, ConfidenceModifier: 0.5,
		})
	}

	items, err := BuildDailyQueue(database, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) > 3 {
		t.Errorf("got %d items, want max 3", len(items))
	}
}

func TestBuildDailyQueue_CorrectPriorityOrder(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database, "topic-high", "topic-high", "High Priority")
	seedTopic(t, database, "topic-low", "topic-low", "Low Priority")

	now := time.Now()

	// High priority: mastery 0, confidence 1.0, high AI exposure count → gap=1.0, freq=1.5
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "topic-high", MasteryLevel: 0, ConfidenceModifier: 1.0,
		AIExposureCount: 10, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Low priority: mastery 2, confidence 1.0, no AI exposure → gap=0.6, freq=1.0
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "topic-low", MasteryLevel: 2, ConfidenceModifier: 1.0,
		AIExposureCount: 0, UpdatedAt: now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	items, err := BuildDailyQueue(database, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].TopicID != "topic-high" {
		t.Errorf("first item = %q, want topic-high", items[0].TopicID)
	}
	if items[0].PriorityScore <= items[1].PriorityScore {
		t.Errorf("first priority (%f) should be > second (%f)", items[0].PriorityScore, items[1].PriorityScore)
	}
}

func TestBuildDailyQueue_AIHeavyTopicsRankHigher(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database, "ai-heavy", "ai-heavy", "AI Heavy")
	seedTopic(t, database, "authored", "authored", "Authored")

	now := time.Now()

	// Both at same mastery level, but AI-heavy has more exposures
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "ai-heavy", MasteryLevel: 1, ConfidenceModifier: 0.5,
		AIExposureCount: 8, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "authored", MasteryLevel: 1, ConfidenceModifier: 0.9,
		AIExposureCount: 0, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	items, err := BuildDailyQueue(database, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("got %d items, want at least 2", len(items))
	}

	// ai-heavy should rank higher due to lower effective mastery + higher frequency weight
	if items[0].TopicID != "ai-heavy" {
		t.Errorf("first item = %q, want ai-heavy (AI-heavy topics should rank higher)", items[0].TopicID)
	}
}
