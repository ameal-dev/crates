package curriculum

import (
	"database/sql"
	"fmt"

	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

// SeedCurriculum walks the topic tree and upserts all topics into the database.
func SeedCurriculum(db *sql.DB, topics []models.Topic) error {
	return seedTopics(db, topics)
}

func seedTopics(db *sql.DB, topics []models.Topic) error {
	for _, t := range topics {
		if err := queries.UpsertTopic(db, t); err != nil {
			return fmt.Errorf("seed topic %s: %w", t.ID, err)
		}
		if len(t.Children) > 0 {
			if err := seedTopics(db, t.Children); err != nil {
				return err
			}
		}
	}
	return nil
}
