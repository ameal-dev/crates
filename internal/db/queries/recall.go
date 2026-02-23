package queries

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// InsertRecallCard saves a new recall card with SM-2 defaults.
func InsertRecallCard(db *sql.DB, id, topicID, question, answer string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO recall_cards (id, topic_id, question, answer, next_review, interval_days, ease_factor, repetitions, created_at)
		 VALUES (?, ?, ?, ?, ?, 1.0, 2.5, 0, ?)`,
		id, topicID, question, answer, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert recall card: %w", err)
	}
	return nil
}

// GetDueRecallCards returns cards due for review (next_review <= now).
func GetDueRecallCards(db *sql.DB, limit int) ([]models.RecallCard, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := db.Query(
		`SELECT id, topic_id, question, answer, next_review, interval_days, ease_factor, repetitions, created_at
		 FROM recall_cards WHERE next_review <= ? ORDER BY next_review LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query due cards: %w", err)
	}
	defer rows.Close()

	return scanRecallCards(rows)
}

// GetDueRecallCardCount returns the number of cards due for review.
func GetDueRecallCardCount(db *sql.DB) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM recall_cards WHERE next_review <= ?`, now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count due cards: %w", err)
	}
	return count, nil
}

// GetTotalRecallCardCount returns the total number of recall cards (regardless of review status).
func GetTotalRecallCardCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM recall_cards`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count total cards: %w", err)
	}
	return count, nil
}

// UpdateRecallCard updates a card's SM-2 scheduling fields after review.
func UpdateRecallCard(db *sql.DB, id string, nextReview time.Time, intervalDays, easeFactor float64, repetitions int) error {
	_, err := db.Exec(
		`UPDATE recall_cards SET next_review = ?, interval_days = ?, ease_factor = ?, repetitions = ? WHERE id = ?`,
		nextReview.UTC().Format(time.RFC3339), intervalDays, easeFactor, repetitions, id,
	)
	if err != nil {
		return fmt.Errorf("update recall card: %w", err)
	}
	return nil
}

// GetAllRecallCards returns all recall cards.
func GetAllRecallCards(db *sql.DB) ([]models.RecallCard, error) {
	rows, err := db.Query(
		`SELECT id, topic_id, question, answer, next_review, interval_days, ease_factor, repetitions, created_at
		 FROM recall_cards ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all cards: %w", err)
	}
	defer rows.Close()
	return scanRecallCards(rows)
}

func scanRecallCards(rows *sql.Rows) ([]models.RecallCard, error) {
	var cards []models.RecallCard
	for rows.Next() {
		var c models.RecallCard
		var nextReview, createdAt string
		if err := rows.Scan(&c.ID, &c.TopicID, &c.Question, &c.Answer, &nextReview, &c.IntervalDays, &c.EaseFactor, &c.Repetitions, &createdAt); err != nil {
			return nil, fmt.Errorf("scan recall card: %w", err)
		}
		c.NextReview, _ = time.Parse(time.RFC3339, nextReview)
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		cards = append(cards, c)
	}
	return cards, rows.Err()
}
