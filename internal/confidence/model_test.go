package confidence

import (
	"math"
	"testing"
	"time"

	"github.com/ameal-dev/crates/internal/db/models"
)

func TestEffectiveMastery(t *testing.T) {
	tests := []struct {
		name     string
		mastery  int
		conf     float64
		expected float64
	}{
		{"basic", 3, 0.8, 2.4},
		{"full confidence", 5, 1.0, 5.0},
		{"zero mastery", 0, 1.0, 0.0},
		{"low confidence", 4, 0.3, 1.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := models.TopicProgress{MasteryLevel: tt.mastery, ConfidenceModifier: tt.conf}
			got := EffectiveMastery(p)
			if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("EffectiveMastery() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestUpdateAfterExposure_AIAccepted(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 1.0, AIExposureCount: 0}
	p = UpdateAfterExposure(p, "AI_ACCEPTED")
	if math.Abs(p.ConfidenceModifier-0.85) > 0.001 {
		t.Errorf("confidence = %f, want 0.85", p.ConfidenceModifier)
	}
	if p.AIExposureCount != 1 {
		t.Errorf("ai_exposure_count = %d, want 1", p.AIExposureCount)
	}
	if p.LastExposureSource == nil || *p.LastExposureSource != "AI_ACCEPTED" {
		t.Errorf("last_exposure_source = %v, want AI_ACCEPTED", p.LastExposureSource)
	}
}

func TestUpdateAfterExposure_AIAccepted_Floor(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.35}
	p = UpdateAfterExposure(p, "AI_ACCEPTED")
	if p.ConfidenceModifier != 0.3 {
		t.Errorf("confidence = %f, want 0.3 (floor)", p.ConfidenceModifier)
	}
}

func TestUpdateAfterExposure_Authored(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.8, AuthoredCount: 0}
	p = UpdateAfterExposure(p, "AUTHORED")
	if math.Abs(p.ConfidenceModifier-0.85) > 0.001 {
		t.Errorf("confidence = %f, want 0.85", p.ConfidenceModifier)
	}
	if p.AuthoredCount != 1 {
		t.Errorf("authored_count = %d, want 1", p.AuthoredCount)
	}
}

func TestUpdateAfterExposure_Authored_Cap(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.98}
	p = UpdateAfterExposure(p, "AUTHORED")
	if p.ConfidenceModifier != 1.0 {
		t.Errorf("confidence = %f, want 1.0 (cap)", p.ConfidenceModifier)
	}
}

func TestUpdateAfterSession(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.5}
	p = UpdateAfterSession(p)
	if p.ConfidenceModifier != 1.0 {
		t.Errorf("confidence = %f, want 1.0", p.ConfidenceModifier)
	}
	if p.LastConfirmedAt == nil {
		t.Error("LastConfirmedAt should be set")
	}
}

func TestUpdateAfterRecall_Quality4(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.8}
	p = UpdateAfterRecall(p, 4)
	if math.Abs(p.ConfidenceModifier-0.9) > 0.001 {
		t.Errorf("confidence = %f, want 0.9", p.ConfidenceModifier)
	}
}

func TestUpdateAfterRecall_Quality2_NoChange(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 0.8}
	p = UpdateAfterRecall(p, 2)
	if p.ConfidenceModifier != 0.8 {
		t.Errorf("confidence = %f, want 0.8 (no change)", p.ConfidenceModifier)
	}
}

func TestApplyTimeDecay(t *testing.T) {
	fiveWeeksAgo := time.Now().Add(-5 * 7 * 24 * time.Hour)
	p := models.TopicProgress{ConfidenceModifier: 1.0, LastConfirmedAt: &fiveWeeksAgo}
	p = ApplyTimeDecay(p, time.Now())
	expected := 1.0 - 5*0.02 // 0.90
	if math.Abs(p.ConfidenceModifier-expected) > 0.001 {
		t.Errorf("confidence = %f, want %f", p.ConfidenceModifier, expected)
	}
}

func TestApplyTimeDecay_NilLastConfirmed(t *testing.T) {
	p := models.TopicProgress{ConfidenceModifier: 1.0}
	p = ApplyTimeDecay(p, time.Now())
	if p.ConfidenceModifier != 1.0 {
		t.Errorf("confidence = %f, want 1.0 (no-op)", p.ConfidenceModifier)
	}
}

func TestApplyTimeDecay_Floor(t *testing.T) {
	longAgo := time.Now().Add(-100 * 7 * 24 * time.Hour)
	p := models.TopicProgress{ConfidenceModifier: 0.5, LastConfirmedAt: &longAgo}
	p = ApplyTimeDecay(p, time.Now())
	if p.ConfidenceModifier != 0.3 {
		t.Errorf("confidence = %f, want 0.3 (floor)", p.ConfidenceModifier)
	}
}
