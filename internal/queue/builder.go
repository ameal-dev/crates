package queue

import (
	"database/sql"
	"math"
	"sort"
	"time"

	"github.com/ameal-dev/crates/internal/confidence"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

const maxDailyQueue = 3

// BuildDailyQueue computes the day's lesson queue from topic progress and exposures.
func BuildDailyQueue(db *sql.DB, now time.Time) ([]models.LessonQueueItem, error) {
	allProgress, err := queries.GetAllTopicProgress(db)
	if err != nil {
		return nil, err
	}

	if len(allProgress) == 0 {
		return nil, nil
	}

	type scored struct {
		topicID  string
		priority float64
	}

	var candidates []scored
	for _, p := range allProgress {
		p = confidence.ApplyTimeDecay(p, now)
		eff := confidence.EffectiveMastery(p)

		// Topics with effective_mastery > 3.0 are recall-only
		if eff > 3.0 {
			continue
		}

		priority := computePriority(p, eff, now)
		if priority > 0 {
			candidates = append(candidates, scored{topicID: p.TopicID, priority: priority})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	limit := maxDailyQueue
	if len(candidates) < limit {
		limit = len(candidates)
	}
	candidates = candidates[:limit]

	// Clear existing queue and write new items
	if err := queries.ClearLessonQueue(db); err != nil {
		return nil, err
	}

	items := make([]models.LessonQueueItem, len(candidates))
	for i, c := range candidates {
		item := models.LessonQueueItem{
			TopicID:       c.topicID,
			PriorityScore: c.priority,
			QueuedAt:      now,
		}
		if err := queries.UpsertLessonQueueItem(db, item); err != nil {
			return nil, err
		}
		items[i] = item
	}

	return items, nil
}

func computePriority(p models.TopicProgress, effectiveMastery float64, now time.Time) float64 {
	gapWeight := gapWeightFor(effectiveMastery)
	recencyWeight := recencyWeightFor(p, now)
	frequencyWeight := frequencyWeightFor(p)

	return gapWeight * recencyWeight * frequencyWeight
}

func gapWeightFor(eff float64) float64 {
	switch {
	case eff == 0:
		return 1.0
	case eff < 1.0:
		return 0.9
	case eff <= 2.0:
		return 0.6
	case eff <= 3.0:
		return 0.3
	default:
		return 0.1
	}
}

func recencyWeightFor(p models.TopicProgress, now time.Time) float64 {
	// Use UpdatedAt as proxy for last exposure time
	daysSince := now.Sub(p.UpdatedAt).Hours() / 24
	if daysSince <= 1 {
		return 2.0
	}
	// Linear decay from 2.0 to 1.0 over 7 days
	decay := math.Max(0, 1.0-(daysSince-1)/6.0)
	return 1.0 + decay
}

func frequencyWeightFor(p models.TopicProgress) float64 {
	// 1.0 + 0.1 * ai_exposure_count, capped at 1.5
	w := 1.0 + 0.1*float64(p.AIExposureCount)
	if w > 1.5 {
		return 1.5
	}
	return w
}
