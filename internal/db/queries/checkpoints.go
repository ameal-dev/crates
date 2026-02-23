package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// InsertCheckpoint saves a knowledge checkpoint.
func InsertCheckpoint(db *sql.DB, id, sessionID, concept, evidence string, masteryDelta int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO checkpoints (id, session_id, concept, evidence, mastery_delta, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, sessionID, concept, evidence, masteryDelta, now,
	)
	if err != nil {
		return fmt.Errorf("insert checkpoint: %w", err)
	}
	return nil
}

// GetCheckpointsForSession returns all checkpoints in a session.
func GetCheckpointsForSession(db *sql.DB, sessionID string) ([]models.Checkpoint, error) {
	rows, err := db.Query(
		`SELECT id, session_id, concept, evidence, mastery_delta, created_at FROM checkpoints WHERE session_id = ? ORDER BY created_at`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query checkpoints: %w", err)
	}
	defer rows.Close()

	var cps []models.Checkpoint
	for rows.Next() {
		var cp models.Checkpoint
		var createdAt string
		if err := rows.Scan(&cp.ID, &cp.SessionID, &cp.Concept, &cp.Evidence, &cp.MasteryDelta, &createdAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		cp.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		cps = append(cps, cp)
	}
	return cps, rows.Err()
}
