package mcp

import (
	"database/sql"
	"testing"
	"time"

	"github.com/ameal-dev/crates/internal/db"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/migrations"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestGetUserContext_Empty(t *testing.T) {
	database := setupTestDB(t)
	s := &Server{db: database}

	_, out, err := s.handleGetUserContext(nil, &mcpsdk.CallToolRequest{}, getUserContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.WeakTopics) != 0 {
		t.Errorf("got %d weak topics, want 0", len(out.WeakTopics))
	}
}

func TestGetUserContext_WeakTopics(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")
	seedTopic(t, database, "go-channels", "go-channels", "Channels")
	seedTopic(t, database, "go-select", "go-select", "Select")

	// go-goroutines: mastery 1, confidence 0.8 → effective 0.8
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-goroutines", MasteryLevel: 1, ConfidenceModifier: 0.8,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// go-channels: mastery 3, confidence 1.0 → effective 3.0 (should be excluded, >= 2.0)
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-channels", MasteryLevel: 3, ConfidenceModifier: 1.0,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// go-select: mastery 2, confidence 0.5 → effective 1.0
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-select", MasteryLevel: 2, ConfidenceModifier: 0.5,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s := &Server{db: database}
	_, out, err := s.handleGetUserContext(nil, &mcpsdk.CallToolRequest{}, getUserContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.WeakTopics) != 2 {
		t.Fatalf("got %d weak topics, want 2", len(out.WeakTopics))
	}

	// Should be sorted ascending by effective mastery
	if out.WeakTopics[0].Slug != "go-goroutines" {
		t.Errorf("first weak topic = %q, want go-goroutines", out.WeakTopics[0].Slug)
	}
	if out.WeakTopics[1].Slug != "go-select" {
		t.Errorf("second weak topic = %q, want go-select", out.WeakTopics[1].Slug)
	}
}

func TestGetUserContext_ExcludesStrong(t *testing.T) {
	database := setupTestDB(t)
	seedTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")

	// mastery 4, confidence 1.0 → effective 4.0 (should be excluded)
	if err := queries.UpsertTopicProgress(database, models.TopicProgress{
		TopicID: "go-goroutines", MasteryLevel: 4, ConfidenceModifier: 1.0,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s := &Server{db: database}
	_, out, err := s.handleGetUserContext(nil, &mcpsdk.CallToolRequest{}, getUserContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.WeakTopics) != 0 {
		t.Errorf("got %d weak topics, want 0 (strong topic excluded)", len(out.WeakTopics))
	}
}

func TestRecordExposure_ValidSource(t *testing.T) {
	database := setupTestDB(t)
	s := &Server{db: database}

	_, out, err := s.handleRecordExposure(nil, &mcpsdk.CallToolRequest{}, recordExposureInput{
		Diff:      "some diff",
		CommitSHA: "abc123",
		Source:    "AUTHORED",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "accepted" {
		t.Errorf("status = %q, want accepted", out.Status)
	}

	// Wait for background goroutine
	s.wg.Wait()
}

func TestRecordExposure_InvalidSource(t *testing.T) {
	database := setupTestDB(t)
	s := &Server{db: database}

	_, _, err := s.handleRecordExposure(nil, &mcpsdk.CallToolRequest{}, recordExposureInput{
		Diff:   "diff",
		Source: "INVALID",
	})
	if err == nil {
		t.Error("expected error for invalid source")
	}
}

func TestGetUserContext_Max5(t *testing.T) {
	database := setupTestDB(t)
	// Create 7 weak topics
	for i := 0; i < 7; i++ {
		id := "topic-" + time.Now().Format("150405") + "-" + string(rune('a'+i))
		seedTopic(t, database, id, id, "Topic "+string(rune('A'+i)))
		_ = queries.UpsertTopicProgress(database, models.TopicProgress{
			TopicID: id, MasteryLevel: 1, ConfidenceModifier: 0.5,
		})
	}

	s := &Server{db: database}
	_, out, err := s.handleGetUserContext(nil, &mcpsdk.CallToolRequest{}, getUserContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.WeakTopics) > 5 {
		t.Errorf("got %d weak topics, want max 5", len(out.WeakTopics))
	}
}
