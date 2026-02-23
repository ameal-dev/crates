package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// UpsertLessonQueueItem inserts or updates a lesson queue entry.
func UpsertLessonQueueItem(db *sql.DB, item models.LessonQueueItem) error {
	_, err := db.Exec(`
		INSERT INTO lesson_queue (topic_id, priority_score, source_exposure_id, queued_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(topic_id) DO UPDATE SET
			priority_score = excluded.priority_score,
			source_exposure_id = excluded.source_exposure_id,
			queued_at = excluded.queued_at
	`, item.TopicID, item.PriorityScore, item.SourceExposureID, item.QueuedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert lesson queue item: %w", err)
	}
	return nil
}

// GetLessonQueue returns the lesson queue ordered by priority descending.
func GetLessonQueue(db *sql.DB, limit int) ([]models.LessonQueueItem, error) {
	rows, err := db.Query(`
		SELECT topic_id, priority_score, source_exposure_id, queued_at
		FROM lesson_queue ORDER BY priority_score DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query lesson queue: %w", err)
	}
	defer rows.Close()

	var results []models.LessonQueueItem
	for rows.Next() {
		var item models.LessonQueueItem
		var sourceExposureID sql.NullString
		var queuedAt string
		if err := rows.Scan(&item.TopicID, &item.PriorityScore, &sourceExposureID, &queuedAt); err != nil {
			return nil, fmt.Errorf("scan lesson queue item: %w", err)
		}
		if sourceExposureID.Valid {
			item.SourceExposureID = &sourceExposureID.String
		}
		item.QueuedAt, _ = time.Parse(time.RFC3339, queuedAt)
		results = append(results, item)
	}
	return results, rows.Err()
}

// ClearLessonQueue removes all items from the lesson queue.
func ClearLessonQueue(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM lesson_queue`)
	if err != nil {
		return fmt.Errorf("clear lesson queue: %w", err)
	}
	return nil
}

// DeleteLessonQueueItem removes a single item from the lesson queue.
func DeleteLessonQueueItem(db *sql.DB, topicID string) error {
	_, err := db.Exec(`DELETE FROM lesson_queue WHERE topic_id = ?`, topicID)
	if err != nil {
		return fmt.Errorf("delete lesson queue item: %w", err)
	}
	return nil
}
