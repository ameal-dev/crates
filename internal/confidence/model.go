package confidence

import (
	"math"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

// EffectiveMastery returns the real mastery signal used for all filtering.
func EffectiveMastery(p models.TopicProgress) float64 {
	return float64(p.MasteryLevel) * p.ConfidenceModifier
}

// UpdateAfterExposure adjusts confidence based on how a concept was encountered in code.
func UpdateAfterExposure(p models.TopicProgress, source string) models.TopicProgress {
	switch source {
	case "AI_ACCEPTED":
		p.ConfidenceModifier -= 0.15
		p.AIExposureCount++
	case "AUTHORED":
		p.ConfidenceModifier += 0.05
		p.AuthoredCount++
	}
	p.ConfidenceModifier = clampConfidence(p.ConfidenceModifier)
	p.LastExposureSource = &source
	return p
}

// UpdateAfterSession resets confidence to 1.0 after a confirmed Socratic session.
func UpdateAfterSession(p models.TopicProgress) models.TopicProgress {
	p.ConfidenceModifier = 1.0
	now := time.Now().UTC()
	p.LastConfirmedAt = &now
	return p
}

// UpdateAfterRecall adjusts confidence after a recall card grading.
func UpdateAfterRecall(p models.TopicProgress, quality int) models.TopicProgress {
	if quality >= 4 {
		p.ConfidenceModifier += 0.1
		p.ConfidenceModifier = clampConfidence(p.ConfidenceModifier)
	}
	return p
}

// ApplyTimeDecay reduces confidence by 0.02 per week since last confirmed session.
func ApplyTimeDecay(p models.TopicProgress, now time.Time) models.TopicProgress {
	if p.LastConfirmedAt == nil {
		return p
	}
	weeks := now.Sub(*p.LastConfirmedAt).Hours() / (24 * 7)
	if weeks <= 0 {
		return p
	}
	decay := 0.02 * math.Floor(weeks)
	p.ConfidenceModifier -= decay
	p.ConfidenceModifier = clampConfidence(p.ConfidenceModifier)
	return p
}

func clampConfidence(v float64) float64 {
	if v < 0.3 {
		return 0.3
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}
