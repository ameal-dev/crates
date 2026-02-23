package queries_test

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

func seedTopics(t *testing.T, database *sql.DB) {
	t.Helper()
	topics := []models.Topic{
		{ID: "topic-a", Slug: "go/basics/variables", Title: "Variables", SortOrder: 1},
		{ID: "topic-b", Slug: "go/basics/functions", Title: "Functions", SortOrder: 2},
		{ID: "topic-c", Slug: "go/concurrency/goroutines", Title: "Goroutines", SortOrder: 3},
	}
	for _, t := range topics {
		if err := queries.UpsertTopic(database, t); err != nil {
			panic(err)
		}
	}
}

func TestGetSessionCountByTopic(t *testing.T) {
	database := setupTestDB(t)
	seedTopics(t, database)

	// Create sessions: 2 completed for topic-a, 1 completed + 1 active for topic-b
	sessions := []struct {
		id, topicID, status string
	}{
		{"s1", "topic-a", "completed"},
		{"s2", "topic-a", "completed"},
		{"s3", "topic-b", "completed"},
		{"s4", "topic-b", "active"},
	}
	for _, s := range sessions {
		if _, err := queries.CreateSession(database, s.id, s.topicID); err != nil {
			t.Fatalf("create session: %v", err)
		}
		if s.status == "completed" {
			if err := queries.UpdateSessionStatus(database, s.id, "completed"); err != nil {
				t.Fatalf("update session status: %v", err)
			}
		}
	}

	counts, err := queries.GetSessionCountByTopic(database)
	if err != nil {
		t.Fatalf("GetSessionCountByTopic: %v", err)
	}

	if counts["topic-a"] != 2 {
		t.Errorf("topic-a: got %d, want 2", counts["topic-a"])
	}
	if counts["topic-b"] != 1 {
		t.Errorf("topic-b: got %d, want 1 (active session should not count)", counts["topic-b"])
	}
	if counts["topic-c"] != 0 {
		t.Errorf("topic-c: got %d, want 0", counts["topic-c"])
	}
}

func TestGetRecallCardCountsByTopic(t *testing.T) {
	database := setupTestDB(t)
	seedTopics(t, database)

	now := time.Now().UTC()
	past := now.Add(-24 * time.Hour)
	future := now.Add(48 * time.Hour)

	// topic-a: 2 cards, 1 due
	if err := queries.InsertRecallCard(database, "c1", "topic-a", "Q1", "A1"); err != nil {
		t.Fatalf("insert card: %v", err)
	}
	if err := queries.UpdateRecallCard(database, "c1", past, 1, 2.5, 1); err != nil {
		t.Fatalf("update card: %v", err)
	}
	if err := queries.InsertRecallCard(database, "c2", "topic-a", "Q2", "A2"); err != nil {
		t.Fatalf("insert card: %v", err)
	}
	if err := queries.UpdateRecallCard(database, "c2", future, 3, 2.5, 2); err != nil {
		t.Fatalf("update card: %v", err)
	}

	// topic-b: 1 card, 1 due
	if err := queries.InsertRecallCard(database, "c3", "topic-b", "Q3", "A3"); err != nil {
		t.Fatalf("insert card: %v", err)
	}
	if err := queries.UpdateRecallCard(database, "c3", past, 1, 2.5, 1); err != nil {
		t.Fatalf("update card: %v", err)
	}

	total, due, err := queries.GetRecallCardCountsByTopic(database)
	if err != nil {
		t.Fatalf("GetRecallCardCountsByTopic: %v", err)
	}

	if total["topic-a"] != 2 {
		t.Errorf("topic-a total: got %d, want 2", total["topic-a"])
	}
	if due["topic-a"] != 1 {
		t.Errorf("topic-a due: got %d, want 1", due["topic-a"])
	}
	if total["topic-b"] != 1 {
		t.Errorf("topic-b total: got %d, want 1", total["topic-b"])
	}
	if due["topic-b"] != 1 {
		t.Errorf("topic-b due: got %d, want 1", due["topic-b"])
	}
	if total["topic-c"] != 0 {
		t.Errorf("topic-c total: got %d, want 0", total["topic-c"])
	}
}
