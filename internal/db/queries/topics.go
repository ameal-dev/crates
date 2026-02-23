package queries

import (
	"database/sql"
	"fmt"

	"github.com/ameal-dev/crates/internal/db/models"
)

// GetTopicTree returns all topics organized as a tree (top-level topics with children populated).
func GetTopicTree(db *sql.DB) ([]models.Topic, error) {
	rows, err := db.Query(`SELECT id, parent_id, slug, title, description, sort_order FROM topics ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	var all []models.Topic
	for rows.Next() {
		var t models.Topic
		if err := rows.Scan(&t.ID, &t.ParentID, &t.Slug, &t.Title, &t.Description, &t.SortOrder); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		all = append(all, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	// Build tree
	byID := make(map[string]*models.Topic, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	var roots []models.Topic
	for i := range all {
		if all[i].ParentID == nil {
			roots = append(roots, all[i])
		} else {
			if parent, ok := byID[*all[i].ParentID]; ok {
				parent.Children = append(parent.Children, all[i])
			}
		}
	}

	// Copy children back to roots
	for i := range roots {
		if node, ok := byID[roots[i].ID]; ok {
			roots[i].Children = node.Children
		}
	}

	return roots, nil
}

// GetTopic returns a single topic by ID.
func GetTopic(db *sql.DB, id string) (models.Topic, error) {
	var t models.Topic
	err := db.QueryRow(
		`SELECT id, parent_id, slug, title, description, sort_order FROM topics WHERE id = ?`, id,
	).Scan(&t.ID, &t.ParentID, &t.Slug, &t.Title, &t.Description, &t.SortOrder)
	if err != nil {
		return t, fmt.Errorf("get topic %s: %w", id, err)
	}
	return t, nil
}

// UpsertTopic inserts or updates a topic.
func UpsertTopic(db *sql.DB, t models.Topic) error {
	_, err := db.Exec(`
		INSERT INTO topics (id, parent_id, slug, title, description, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			parent_id = excluded.parent_id,
			slug = excluded.slug,
			title = excluded.title,
			description = excluded.description,
			sort_order = excluded.sort_order
	`, t.ID, t.ParentID, t.Slug, t.Title, t.Description, t.SortOrder)
	if err != nil {
		return fmt.Errorf("upsert topic %s: %w", t.ID, err)
	}
	return nil
}

// GetTopicChildren returns direct children of a topic.
func GetTopicChildren(db *sql.DB, parentID string) ([]models.Topic, error) {
	rows, err := db.Query(
		`SELECT id, parent_id, slug, title, description, sort_order FROM topics WHERE parent_id = ? ORDER BY sort_order`,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	var topics []models.Topic
	for rows.Next() {
		var t models.Topic
		if err := rows.Scan(&t.ID, &t.ParentID, &t.Slug, &t.Title, &t.Description, &t.SortOrder); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}
