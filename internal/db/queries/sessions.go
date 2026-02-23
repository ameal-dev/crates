package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// CreateSession creates a new learning session.
func CreateSession(db *sql.DB, id, topicID string) (models.Session, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO sessions (id, topic_id, started_at, last_activity_at, status) VALUES (?, ?, ?, ?, 'active')`,
		id, topicID, now, now,
	)
	if err != nil {
		return models.Session{}, fmt.Errorf("create session: %w", err)
	}
	return GetSession(db, id)
}

// GetSession returns a session by ID.
func GetSession(db *sql.DB, id string) (models.Session, error) {
	var s models.Session
	var startedAt, lastActivity, status string
	err := db.QueryRow(
		`SELECT id, topic_id, started_at, last_activity_at, status FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.TopicID, &startedAt, &lastActivity, &status)
	if err != nil {
		return s, fmt.Errorf("get session %s: %w", id, err)
	}
	s.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	s.LastActivityAt, _ = time.Parse(time.RFC3339, lastActivity)
	s.Status = status
	return s, nil
}

// UpdateSessionActivity updates the last_activity_at timestamp.
func UpdateSessionActivity(db *sql.DB, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`UPDATE sessions SET last_activity_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("update session activity: %w", err)
	}
	return nil
}

// UpdateSessionStatus sets the session status.
func UpdateSessionStatus(db *sql.DB, id, status string) error {
	_, err := db.Exec(`UPDATE sessions SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	return nil
}

// GetRecentSessions returns the most recent sessions with their topic titles.
func GetRecentSessions(db *sql.DB, limit int) ([]models.Session, error) {
	rows, err := db.Query(`
		SELECT s.id, s.topic_id, s.started_at, s.last_activity_at, s.status
		FROM sessions s
		ORDER BY s.last_activity_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		var startedAt, lastActivity string
		if err := rows.Scan(&s.ID, &s.TopicID, &startedAt, &lastActivity, &s.Status); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		s.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		s.LastActivityAt, _ = time.Parse(time.RFC3339, lastActivity)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetDistinctCompletedSessionDates returns distinct dates (YYYY-MM-DD) of completed sessions, most recent first.
func GetDistinctCompletedSessionDates(db *sql.DB, limit int) ([]string, error) {
	rows, err := db.Query(
		`SELECT DISTINCT DATE(started_at) FROM sessions WHERE status = 'completed' ORDER BY 1 DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query completed session dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan session date: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}
