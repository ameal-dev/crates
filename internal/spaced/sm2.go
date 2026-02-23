package spaced

import (
	"math"
	"time"
)

// SM2Result holds the computed scheduling values after a review.
type SM2Result struct {
	NextReview   time.Time
	IntervalDays float64
	EaseFactor   float64
	Repetitions  int
}

// SM2 implements the SM-2 spaced repetition algorithm.
// Quality is a grade from 0-5:
//
//	0 - complete blackout
//	1 - incorrect, but remembered upon seeing answer
//	2 - incorrect, but answer seemed easy to recall
//	3 - correct with serious difficulty
//	4 - correct with some hesitation
//	5 - perfect recall
func SM2(quality int, repetitions int, easeFactor float64, intervalDays float64, now time.Time) SM2Result {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	// Clamp ease factor floor
	const minEase = 1.3

	if quality < 3 {
		// Failed recall — reset
		return SM2Result{
			NextReview:   now.Add(24 * time.Hour), // Review again tomorrow
			IntervalDays: 1.0,
			EaseFactor:   math.Max(minEase, easeFactor-0.2),
			Repetitions:  0,
		}
	}

	// Successful recall
	newEF := easeFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if newEF < minEase {
		newEF = minEase
	}

	var newInterval float64
	newReps := repetitions + 1

	switch {
	case repetitions == 0:
		newInterval = 1.0
	case repetitions == 1:
		newInterval = 6.0
	default:
		newInterval = intervalDays * newEF
	}

	// Round to nearest 0.1 days
	newInterval = math.Round(newInterval*10) / 10

	nextReview := now.Add(time.Duration(newInterval*24) * time.Hour)

	return SM2Result{
		NextReview:   nextReview,
		IntervalDays: newInterval,
		EaseFactor:   math.Round(newEF*100) / 100, // round to 2 decimal places
		Repetitions:  newReps,
	}
}
