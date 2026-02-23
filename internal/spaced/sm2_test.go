package spaced

import (
	"math"
	"testing"
	"time"
)

func TestSM2(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		quality      int
		repetitions  int
		easeFactor   float64
		intervalDays float64
		wantInterval float64
		wantEF       float64
		wantReps     int
	}{
		{
			name:         "first review perfect",
			quality:      5,
			repetitions:  0,
			easeFactor:   2.5,
			intervalDays: 0,
			wantInterval: 1.0,
			wantEF:       2.6,
			wantReps:     1,
		},
		{
			name:         "second review perfect",
			quality:      5,
			repetitions:  1,
			easeFactor:   2.6,
			intervalDays: 1.0,
			wantInterval: 6.0,
			wantEF:       2.7,
			wantReps:     2,
		},
		{
			name:         "third review perfect",
			quality:      5,
			repetitions:  2,
			easeFactor:   2.7,
			intervalDays: 6.0,
			wantInterval: 16.8, // 6 * 2.8
			wantEF:       2.8,
			wantReps:     3,
		},
		{
			name:         "review quality 3 (hard)",
			quality:      3,
			repetitions:  2,
			easeFactor:   2.5,
			intervalDays: 6.0,
			wantInterval: 14.2, // 6 * 2.36
			wantEF:       2.36,
			wantReps:     3,
		},
		{
			name:         "review quality 4 (good)",
			quality:      4,
			repetitions:  2,
			easeFactor:   2.5,
			intervalDays: 6.0,
			wantInterval: 15.0, // 6 * 2.5
			wantEF:       2.5,
			wantReps:     3,
		},
		{
			name:         "failed recall resets",
			quality:      2,
			repetitions:  5,
			easeFactor:   2.5,
			intervalDays: 30.0,
			wantInterval: 1.0,
			wantEF:       2.3,
			wantReps:     0,
		},
		{
			name:         "complete blackout resets",
			quality:      0,
			repetitions:  3,
			easeFactor:   2.5,
			intervalDays: 15.0,
			wantInterval: 1.0,
			wantEF:       2.3,
			wantReps:     0,
		},
		{
			name:         "ease factor floor at 1.3",
			quality:      3,
			repetitions:  0,
			easeFactor:   1.3,
			intervalDays: 0,
			wantInterval: 1.0,
			wantEF:       1.3, // Can't go below 1.3
			wantReps:     1,
		},
		{
			name:         "ease factor floor on failure",
			quality:      1,
			repetitions:  2,
			easeFactor:   1.4,
			intervalDays: 6.0,
			wantInterval: 1.0,
			wantEF:       1.3, // Clamped at min
			wantReps:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SM2(tt.quality, tt.repetitions, tt.easeFactor, tt.intervalDays, now)

			if math.Abs(result.IntervalDays-tt.wantInterval) > 0.11 {
				t.Errorf("IntervalDays = %v, want %v", result.IntervalDays, tt.wantInterval)
			}

			if math.Abs(result.EaseFactor-tt.wantEF) > 0.011 {
				t.Errorf("EaseFactor = %v, want %v", result.EaseFactor, tt.wantEF)
			}

			if result.Repetitions != tt.wantReps {
				t.Errorf("Repetitions = %d, want %d", result.Repetitions, tt.wantReps)
			}

			if result.NextReview.Before(now) {
				t.Error("NextReview is in the past")
			}
		})
	}
}

func TestSM2QualityClamping(t *testing.T) {
	now := time.Now()

	// Quality below 0 should be treated as 0
	r1 := SM2(-1, 0, 2.5, 1.0, now)
	if r1.Repetitions != 0 {
		t.Error("negative quality should reset repetitions")
	}

	// Quality above 5 should be treated as 5
	r2 := SM2(10, 0, 2.5, 1.0, now)
	if r2.Repetitions != 1 {
		t.Error("quality > 5 should be treated as success")
	}
}
