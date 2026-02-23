package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ameal-dev/crates/internal/db/queries"
)

// Export writes all user data as JSON to w.
func Export(db *sql.DB, w io.Writer) error {
	sessions, err := queries.GetRecentSessions(db, 1000)
	if err != nil {
		return fmt.Errorf("export sessions: %w", err)
	}

	progress, err := queries.GetAllTopicProgress(db)
	if err != nil {
		return fmt.Errorf("export progress: %w", err)
	}

	cards, err := queries.GetAllRecallCards(db)
	if err != nil {
		return fmt.Errorf("export recall cards: %w", err)
	}

	data := map[string]any{
		"sessions":     sessions,
		"progress":     progress,
		"recall_cards": cards,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
