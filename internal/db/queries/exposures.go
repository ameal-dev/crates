package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// InsertExposure inserts a concept exposure record.
func InsertExposure(db *sql.DB, e models.ConceptExposure) error {
	_, err := db.Exec(`
		INSERT INTO concept_exposures (id, topic_id, source, commit_sha, file_path, code_snippet, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.TopicID, e.Source, e.CommitSHA, e.FilePath, e.CodeSnippet, e.Confidence, e.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert exposure: %w", err)
	}
	return nil
}

// GetExposuresForTopic returns recent exposures for a given topic.
func GetExposuresForTopic(db *sql.DB, topicID string, limit int) ([]models.ConceptExposure, error) {
	rows, err := db.Query(`
		SELECT id, topic_id, source, commit_sha, file_path, code_snippet, confidence, created_at
		FROM concept_exposures WHERE topic_id = ? ORDER BY created_at DESC LIMIT ?
	`, topicID, limit)
	if err != nil {
		return nil, fmt.Errorf("query exposures for topic: %w", err)
	}
	defer rows.Close()
	return scanExposures(rows)
}

// GetRecentExposures returns the most recent exposures across all topics.
func GetRecentExposures(db *sql.DB, since time.Time, limit int) ([]models.ConceptExposure, error) {
	rows, err := db.Query(`
		SELECT id, topic_id, source, commit_sha, file_path, code_snippet, confidence, created_at
		FROM concept_exposures WHERE created_at >= ? ORDER BY created_at DESC LIMIT ?
	`, since.UTC().Format(time.RFC3339), limit)
	if err != nil {
		return nil, fmt.Errorf("query recent exposures: %w", err)
	}
	defer rows.Close()
	return scanExposures(rows)
}

// CountExposuresSince counts exposures for a topic since a given time, optionally filtered by source.
func CountExposuresSince(db *sql.DB, topicID string, since time.Time, source string) (int, error) {
	var count int
	var err error
	if source == "" {
		err = db.QueryRow(`
			SELECT COUNT(*) FROM concept_exposures WHERE topic_id = ? AND created_at >= ?
		`, topicID, since.UTC().Format(time.RFC3339)).Scan(&count)
	} else {
		err = db.QueryRow(`
			SELECT COUNT(*) FROM concept_exposures WHERE topic_id = ? AND created_at >= ? AND source = ?
		`, topicID, since.UTC().Format(time.RFC3339), source).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count exposures: %w", err)
	}
	return count, nil
}

func scanExposures(rows *sql.Rows) ([]models.ConceptExposure, error) {
	var results []models.ConceptExposure
	for rows.Next() {
		var e models.ConceptExposure
		var commitSHA, filePath, codeSnippet sql.NullString
		var createdAt string
		if err := rows.Scan(&e.ID, &e.TopicID, &e.Source, &commitSHA, &filePath, &codeSnippet, &e.Confidence, &createdAt); err != nil {
			return nil, fmt.Errorf("scan exposure: %w", err)
		}
		if commitSHA.Valid {
			e.CommitSHA = &commitSHA.String
		}
		if filePath.Valid {
			e.FilePath = &filePath.String
		}
		if codeSnippet.Valid {
			e.CodeSnippet = &codeSnippet.String
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, e)
	}
	return results, rows.Err()
}
