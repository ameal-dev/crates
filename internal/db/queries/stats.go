package queries

import (
	"database/sql"
	"fmt"
	"time"
)

// GetSessionCountByTopic returns map[topicID]count for completed sessions.
func GetSessionCountByTopic(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT topic_id, COUNT(*) FROM sessions WHERE status = 'completed' GROUP BY topic_id`)
	if err != nil {
		return nil, fmt.Errorf("query session counts: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var topicID string
		var count int
		if err := rows.Scan(&topicID, &count); err != nil {
			return nil, fmt.Errorf("scan session count: %w", err)
		}
		result[topicID] = count
	}
	return result, rows.Err()
}

// GetRecallCardCountsByTopic returns (total map, due map) per topic.
func GetRecallCardCountsByTopic(db *sql.DB) (total map[string]int, due map[string]int, err error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := db.Query(`
		SELECT topic_id, COUNT(*), SUM(CASE WHEN next_review <= ? THEN 1 ELSE 0 END)
		FROM recall_cards GROUP BY topic_id
	`, now)
	if err != nil {
		return nil, nil, fmt.Errorf("query recall card counts: %w", err)
	}
	defer rows.Close()

	total = make(map[string]int)
	due = make(map[string]int)
	for rows.Next() {
		var topicID string
		var totalCount int
		var dueCount int
		if err := rows.Scan(&topicID, &totalCount, &dueCount); err != nil {
			return nil, nil, fmt.Errorf("scan recall card count: %w", err)
		}
		total[topicID] = totalCount
		due[topicID] = dueCount
	}
	return total, due, rows.Err()
}
