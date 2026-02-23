package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// GetTopicProgress returns progress for a topic, or zero values if none exists.
func GetTopicProgress(db *sql.DB, topicID string) (models.TopicProgress, error) {
	var p models.TopicProgress
	var lastSession sql.NullString
	var updatedAt string
	var lastExposureSource sql.NullString
	var lastConfirmedAt sql.NullString
	err := db.QueryRow(
		`SELECT topic_id, mastery_level, hints_used, skips, last_session_at, updated_at,
			ai_exposure_count, authored_count, last_exposure_source, confidence_modifier, last_confirmed_at
		FROM topic_progress WHERE topic_id = ?`,
		topicID,
	).Scan(&p.TopicID, &p.MasteryLevel, &p.HintsUsed, &p.Skips, &lastSession, &updatedAt,
		&p.AIExposureCount, &p.AuthoredCount, &lastExposureSource, &p.ConfidenceModifier, &lastConfirmedAt)
	if err == sql.ErrNoRows {
		return models.TopicProgress{TopicID: topicID, ConfidenceModifier: 1.0}, nil
	}
	if err != nil {
		return p, fmt.Errorf("get progress: %w", err)
	}
	if lastSession.Valid {
		t, _ := time.Parse(time.RFC3339, lastSession.String)
		p.LastSessionAt = &t
	}
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastExposureSource.Valid {
		p.LastExposureSource = &lastExposureSource.String
	}
	if lastConfirmedAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastConfirmedAt.String)
		p.LastConfirmedAt = &t
	}
	return p, nil
}

// UpsertTopicProgress inserts or updates topic progress.
func UpsertTopicProgress(db *sql.DB, p models.TopicProgress) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var lastSession *string
	if p.LastSessionAt != nil {
		s := p.LastSessionAt.Format(time.RFC3339)
		lastSession = &s
	}
	var lastConfirmedAt *string
	if p.LastConfirmedAt != nil {
		s := p.LastConfirmedAt.Format(time.RFC3339)
		lastConfirmedAt = &s
	}
	_, err := db.Exec(`
		INSERT INTO topic_progress (topic_id, mastery_level, hints_used, skips, last_session_at, updated_at,
			ai_exposure_count, authored_count, last_exposure_source, confidence_modifier, last_confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(topic_id) DO UPDATE SET
			mastery_level = excluded.mastery_level,
			hints_used = excluded.hints_used,
			skips = excluded.skips,
			last_session_at = excluded.last_session_at,
			updated_at = excluded.updated_at,
			ai_exposure_count = excluded.ai_exposure_count,
			authored_count = excluded.authored_count,
			last_exposure_source = excluded.last_exposure_source,
			confidence_modifier = excluded.confidence_modifier,
			last_confirmed_at = excluded.last_confirmed_at
	`, p.TopicID, p.MasteryLevel, p.HintsUsed, p.Skips, lastSession, now,
		p.AIExposureCount, p.AuthoredCount, p.LastExposureSource, p.ConfidenceModifier, lastConfirmedAt)
	if err != nil {
		return fmt.Errorf("upsert progress: %w", err)
	}
	return nil
}

// IncrementMastery increases the mastery level by delta, capped at 5.
func IncrementMastery(db *sql.DB, topicID string, delta int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO topic_progress (topic_id, mastery_level, updated_at)
		VALUES (?, MIN(?, 5), ?)
		ON CONFLICT(topic_id) DO UPDATE SET
			mastery_level = MIN(topic_progress.mastery_level + ?, 5),
			updated_at = ?
	`, topicID, delta, now, delta, now)
	if err != nil {
		return fmt.Errorf("increment mastery: %w", err)
	}
	return nil
}

// GetAllTopicProgress returns progress for all topics that have progress entries.
func GetAllTopicProgress(db *sql.DB) ([]models.TopicProgress, error) {
	rows, err := db.Query(`SELECT topic_id, mastery_level, hints_used, skips, last_session_at, updated_at,
		ai_exposure_count, authored_count, last_exposure_source, confidence_modifier, last_confirmed_at
		FROM topic_progress`)
	if err != nil {
		return nil, fmt.Errorf("query all progress: %w", err)
	}
	defer rows.Close()

	var results []models.TopicProgress
	for rows.Next() {
		var p models.TopicProgress
		var lastSession sql.NullString
		var updatedAt string
		var lastExposureSource sql.NullString
		var lastConfirmedAt sql.NullString
		if err := rows.Scan(&p.TopicID, &p.MasteryLevel, &p.HintsUsed, &p.Skips, &lastSession, &updatedAt,
			&p.AIExposureCount, &p.AuthoredCount, &lastExposureSource, &p.ConfidenceModifier, &lastConfirmedAt); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		if lastSession.Valid {
			t, _ := time.Parse(time.RFC3339, lastSession.String)
			p.LastSessionAt = &t
		}
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if lastExposureSource.Valid {
			p.LastExposureSource = &lastExposureSource.String
		}
		if lastConfirmedAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastConfirmedAt.String)
			p.LastConfirmedAt = &t
		}
		results = append(results, p)
	}
	return results, rows.Err()
}
