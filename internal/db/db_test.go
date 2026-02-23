package db_test

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

func seedTestTopic(t *testing.T, database *sql.DB, id, slug, title string) {
	t.Helper()
	err := queries.UpsertTopic(database, models.Topic{
		ID: id, Slug: slug, Title: title, Description: "test", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
}

func TestOpenAndMigrate(t *testing.T) {
	database := setupTestDB(t)

	tables := []string{"topics", "sessions", "messages", "checkpoints", "topic_progress", "recall_cards", "app_state", "concept_exposures", "lesson_queue"}
	for _, table := range tables {
		var count int
		err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s not accessible: %v", table, err)
		}
	}
}

func TestTopicCRUD(t *testing.T) {
	database := setupTestDB(t)

	topic := models.Topic{
		ID: "go", Slug: "go", Title: "Go", Description: "The Go language", SortOrder: 1,
	}
	if err := queries.UpsertTopic(database, topic); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := queries.GetTopic(database, "go")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Go" {
		t.Errorf("got title %q, want %q", got.Title, "Go")
	}

	topic.Title = "Golang"
	if err := queries.UpsertTopic(database, topic); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	got, err = queries.GetTopic(database, "go")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != "Golang" {
		t.Errorf("got title %q, want %q", got.Title, "Golang")
	}
}

func TestTopicTree(t *testing.T) {
	database := setupTestDB(t)

	parent := "go"
	topics := []models.Topic{
		{ID: "go", Slug: "go", Title: "Go", SortOrder: 1},
		{ID: "go-basics", ParentID: &parent, Slug: "go-basics", Title: "Basics", SortOrder: 1},
		{ID: "go-conc", ParentID: &parent, Slug: "go-concurrency", Title: "Concurrency", SortOrder: 2},
	}
	for _, topic := range topics {
		if err := queries.UpsertTopic(database, topic); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	tree, err := queries.GetTopicTree(database)
	if err != nil {
		t.Fatalf("get tree: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("got %d roots, want 1", len(tree))
	}
	if len(tree[0].Children) != 2 {
		t.Errorf("got %d children, want 2", len(tree[0].Children))
	}
}

func TestSessionLifecycle(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go", "go", "Go")

	sess, err := queries.CreateSession(database, "s1", "go")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sess.Status != "active" {
		t.Errorf("got status %q, want %q", sess.Status, "active")
	}

	time.Sleep(10 * time.Millisecond)
	if err := queries.UpdateSessionActivity(database, "s1"); err != nil {
		t.Fatalf("update activity: %v", err)
	}

	got, err := queries.GetSession(database, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TopicID != "go" {
		t.Errorf("got topic %q, want %q", got.TopicID, "go")
	}

	recent, err := queries.GetRecentSessions(database, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("got %d recent, want 1", len(recent))
	}
}

func TestMessages(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go", "go", "Go")
	if _, err := queries.CreateSession(database, "s1", "go"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := queries.InsertMessage(database, "m1", "s1", "user", "Hello"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := queries.InsertMessage(database, "m2", "s1", "assistant", "Hi there"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	msgs, err := queries.GetMessagesForSession(database, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Error("messages not in expected order")
	}
}

func TestCheckpoints(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go", "go", "Go")
	if _, err := queries.CreateSession(database, "s1", "go"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := queries.InsertCheckpoint(database, "cp1", "s1", "goroutines", "explained correctly", 1); err != nil {
		t.Fatalf("insert: %v", err)
	}

	cps, err := queries.GetCheckpointsForSession(database, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(cps) != 1 {
		t.Fatalf("got %d checkpoints, want 1", len(cps))
	}
	if cps[0].Concept != "goroutines" {
		t.Errorf("got concept %q, want %q", cps[0].Concept, "goroutines")
	}
}

func TestTopicProgress(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go", "go", "Go")

	prog, err := queries.GetTopicProgress(database, "go")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if prog.MasteryLevel != 0 {
		t.Errorf("got mastery %d, want 0", prog.MasteryLevel)
	}

	if err := queries.IncrementMastery(database, "go", 2); err != nil {
		t.Fatalf("increment: %v", err)
	}
	prog, err = queries.GetTopicProgress(database, "go")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if prog.MasteryLevel != 2 {
		t.Errorf("got mastery %d, want 2", prog.MasteryLevel)
	}

	if err := queries.IncrementMastery(database, "go", 10); err != nil {
		t.Fatalf("increment: %v", err)
	}
	prog, err = queries.GetTopicProgress(database, "go")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if prog.MasteryLevel != 5 {
		t.Errorf("got mastery %d, want 5 (capped)", prog.MasteryLevel)
	}
}

func TestRecallCards(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go", "go", "Go")

	// Total count should be 0 initially
	total, err := queries.GetTotalRecallCardCount(database)
	if err != nil {
		t.Fatalf("total count: %v", err)
	}
	if total != 0 {
		t.Errorf("got total %d, want 0", total)
	}

	if err := queries.InsertRecallCard(database, "rc1", "go", "What is a goroutine?", "A lightweight thread"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	due, err := queries.GetDueRecallCards(database, 10)
	if err != nil {
		t.Fatalf("get due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("got %d due cards, want 1", len(due))
	}

	count, err := queries.GetDueRecallCardCount(database)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("got count %d, want 1", count)
	}

	// Total count should be 1
	total, err = queries.GetTotalRecallCardCount(database)
	if err != nil {
		t.Fatalf("total count: %v", err)
	}
	if total != 1 {
		t.Errorf("got total %d, want 1", total)
	}

	future := time.Now().Add(24 * time.Hour)
	if err := queries.UpdateRecallCard(database, "rc1", future, 2.0, 2.6, 1); err != nil {
		t.Fatalf("update: %v", err)
	}

	due, err = queries.GetDueRecallCards(database, 10)
	if err != nil {
		t.Fatalf("get due after update: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("got %d due cards, want 0", len(due))
	}

	// Total count should still be 1 even though none are due
	total, err = queries.GetTotalRecallCardCount(database)
	if err != nil {
		t.Fatalf("total count after update: %v", err)
	}
	if total != 1 {
		t.Errorf("got total %d, want 1", total)
	}
}

func TestAppState(t *testing.T) {
	database := setupTestDB(t)

	// Get non-existent key returns empty string
	val, err := queries.GetAppState(database, "onboarding_completed")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if val != "" {
		t.Errorf("got %q, want empty string", val)
	}

	// Set a value
	if err := queries.SetAppState(database, "onboarding_completed", "true"); err != nil {
		t.Fatalf("set: %v", err)
	}

	val, err = queries.GetAppState(database, "onboarding_completed")
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if val != "true" {
		t.Errorf("got %q, want %q", val, "true")
	}

	// Upsert overwrites
	if err := queries.SetAppState(database, "onboarding_completed", "false"); err != nil {
		t.Fatalf("set overwrite: %v", err)
	}

	val, err = queries.GetAppState(database, "onboarding_completed")
	if err != nil {
		t.Fatalf("get after overwrite: %v", err)
	}
	if val != "false" {
		t.Errorf("got %q, want %q", val, "false")
	}
}

func TestConceptExposures(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")

	now := time.Now().UTC()
	commitSHA := "abc123"
	filePath := "main.go"
	snippet := "go func() { ... }()"

	exposure := models.ConceptExposure{
		ID:          "exp1",
		TopicID:     "go-goroutines",
		Source:      "AI_ACCEPTED",
		CommitSHA:   &commitSHA,
		FilePath:    &filePath,
		CodeSnippet: &snippet,
		Confidence:  0.85,
		CreatedAt:   now,
	}

	if err := queries.InsertExposure(database, exposure); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Query by topic
	exps, err := queries.GetExposuresForTopic(database, "go-goroutines", 10)
	if err != nil {
		t.Fatalf("get for topic: %v", err)
	}
	if len(exps) != 1 {
		t.Fatalf("got %d exposures, want 1", len(exps))
	}
	if exps[0].Source != "AI_ACCEPTED" {
		t.Errorf("got source %q, want %q", exps[0].Source, "AI_ACCEPTED")
	}
	if exps[0].Confidence != 0.85 {
		t.Errorf("got confidence %f, want 0.85", exps[0].Confidence)
	}
	if exps[0].CommitSHA == nil || *exps[0].CommitSHA != "abc123" {
		t.Errorf("got commit_sha %v, want %q", exps[0].CommitSHA, "abc123")
	}

	// Query recent
	recent, err := queries.GetRecentExposures(database, now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("get recent: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("got %d recent, want 1", len(recent))
	}

	// Count since
	count, err := queries.CountExposuresSince(database, "go-goroutines", now.Add(-time.Hour), "AI_ACCEPTED")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("got count %d, want 1", count)
	}

	// Count with different source
	count, err = queries.CountExposuresSince(database, "go-goroutines", now.Add(-time.Hour), "AUTHORED")
	if err != nil {
		t.Fatalf("count authored: %v", err)
	}
	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	// Exposure with nil optional fields
	exposure2 := models.ConceptExposure{
		ID:         "exp2",
		TopicID:    "go-goroutines",
		Source:     "AUTHORED",
		Confidence: 0.9,
		CreatedAt:  now,
	}
	if err := queries.InsertExposure(database, exposure2); err != nil {
		t.Fatalf("insert without optionals: %v", err)
	}

	exps, err = queries.GetExposuresForTopic(database, "go-goroutines", 10)
	if err != nil {
		t.Fatalf("get after second insert: %v", err)
	}
	if len(exps) != 2 {
		t.Fatalf("got %d exposures, want 2", len(exps))
	}
}

func TestLessonQueue(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")
	seedTestTopic(t, database, "go-channels", "go-channels", "Channels")

	now := time.Now().UTC()

	// Empty queue
	items, err := queries.GetLessonQueue(database, 10)
	if err != nil {
		t.Fatalf("get empty queue: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}

	// Upsert items
	if err := queries.UpsertLessonQueueItem(database, models.LessonQueueItem{
		TopicID: "go-goroutines", PriorityScore: 1.5, QueuedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := queries.UpsertLessonQueueItem(database, models.LessonQueueItem{
		TopicID: "go-channels", PriorityScore: 2.0, QueuedAt: now,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Get queue — ordered by priority desc
	items, err = queries.GetLessonQueue(database, 10)
	if err != nil {
		t.Fatalf("get queue: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].TopicID != "go-channels" {
		t.Errorf("first item topic %q, want %q", items[0].TopicID, "go-channels")
	}

	// Upsert same topic — updates priority
	if err := queries.UpsertLessonQueueItem(database, models.LessonQueueItem{
		TopicID: "go-goroutines", PriorityScore: 3.0, QueuedAt: now,
	}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	items, err = queries.GetLessonQueue(database, 10)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if items[0].TopicID != "go-goroutines" {
		t.Errorf("first item topic %q, want %q after update", items[0].TopicID, "go-goroutines")
	}

	// Delete one item
	if err := queries.DeleteLessonQueueItem(database, "go-goroutines"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, err = queries.GetLessonQueue(database, 10)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items after delete, want 1", len(items))
	}

	// Clear queue
	if err := queries.ClearLessonQueue(database); err != nil {
		t.Fatalf("clear: %v", err)
	}
	items, err = queries.GetLessonQueue(database, 10)
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items after clear, want 0", len(items))
	}
}

func TestTopicProgressNewColumns(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")

	// No-row case: confidence_modifier should be 1.0
	prog, err := queries.GetTopicProgress(database, "go-goroutines")
	if err != nil {
		t.Fatalf("get no-row: %v", err)
	}
	if prog.ConfidenceModifier != 1.0 {
		t.Errorf("no-row confidence_modifier = %f, want 1.0", prog.ConfidenceModifier)
	}

	// Set confidence_modifier to 0.8 and read back
	now := time.Now().UTC()
	source := "AI_ACCEPTED"
	prog = models.TopicProgress{
		TopicID:            "go-goroutines",
		MasteryLevel:       3,
		ConfidenceModifier: 0.8,
		AIExposureCount:    5,
		AuthoredCount:      2,
		LastExposureSource: &source,
		LastConfirmedAt:    &now,
	}
	if err := queries.UpsertTopicProgress(database, prog); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := queries.GetTopicProgress(database, "go-goroutines")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ConfidenceModifier != 0.8 {
		t.Errorf("confidence_modifier = %f, want 0.8", got.ConfidenceModifier)
	}
	if got.AIExposureCount != 5 {
		t.Errorf("ai_exposure_count = %d, want 5", got.AIExposureCount)
	}
	if got.AuthoredCount != 2 {
		t.Errorf("authored_count = %d, want 2", got.AuthoredCount)
	}
	if got.LastExposureSource == nil || *got.LastExposureSource != "AI_ACCEPTED" {
		t.Errorf("last_exposure_source = %v, want AI_ACCEPTED", got.LastExposureSource)
	}
	if got.LastConfirmedAt == nil {
		t.Error("last_confirmed_at should not be nil")
	}
}

func TestIncrementMasteryPreservesNewColumns(t *testing.T) {
	database := setupTestDB(t)
	seedTestTopic(t, database, "go-goroutines", "go-goroutines", "Goroutines")

	// Insert progress with custom confidence_modifier
	source := "AI_ACCEPTED"
	prog := models.TopicProgress{
		TopicID:            "go-goroutines",
		MasteryLevel:       2,
		ConfidenceModifier: 0.7,
		AIExposureCount:    3,
		LastExposureSource: &source,
	}
	if err := queries.UpsertTopicProgress(database, prog); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// IncrementMastery only touches mastery_level and updated_at
	if err := queries.IncrementMastery(database, "go-goroutines", 1); err != nil {
		t.Fatalf("increment: %v", err)
	}

	got, err := queries.GetTopicProgress(database, "go-goroutines")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.MasteryLevel != 3 {
		t.Errorf("mastery_level = %d, want 3", got.MasteryLevel)
	}
	if got.ConfidenceModifier != 0.7 {
		t.Errorf("confidence_modifier = %f, want 0.7 (should be preserved)", got.ConfidenceModifier)
	}
	if got.AIExposureCount != 3 {
		t.Errorf("ai_exposure_count = %d, want 3 (should be preserved)", got.AIExposureCount)
	}
}
