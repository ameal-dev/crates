package queries

import (
	"database/sql"
	"fmt"
	"time"
)

// GetAppState returns the value for a key, or "" if not found.
func GetAppState(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get app state %s: %w", key, err)
	}
	return value, nil
}

// SetAppState inserts or updates a key-value pair.
func SetAppState(db *sql.DB, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value, now)
	if err != nil {
		return fmt.Errorf("set app state %s: %w", key, err)
	}
	return nil
}
