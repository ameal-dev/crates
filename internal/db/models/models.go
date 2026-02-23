package models

import "time"

type Topic struct {
	ID          string
	ParentID    *string
	Slug        string
	Title       string
	Description string
	SortOrder   int
	CreatedAt   time.Time
	Children    []Topic // populated in-memory, not stored
}

type Session struct {
	ID             string
	TopicID        string
	StartedAt      time.Time
	LastActivityAt time.Time
	Status         string // "active", "completed", "abandoned"
}

type Message struct {
	ID        string
	SessionID string
	Role      string // "user", "assistant", "system"
	Content   string
	CreatedAt time.Time
}

type Checkpoint struct {
	ID           string
	SessionID    string
	Concept      string
	Evidence     string
	MasteryDelta int
	CreatedAt    time.Time
}

type TopicProgress struct {
	TopicID            string
	MasteryLevel       int // 0-5
	HintsUsed         int
	Skips             int
	LastSessionAt      *time.Time
	UpdatedAt         time.Time
	AIExposureCount    int
	AuthoredCount      int
	LastExposureSource *string
	ConfidenceModifier float64
	LastConfirmedAt    *time.Time
}

type RecallCard struct {
	ID           string
	TopicID      string
	Question     string
	Answer       string
	NextReview   time.Time
	IntervalDays float64
	EaseFactor   float64
	Repetitions  int
	CreatedAt    time.Time
}

type ConceptExposure struct {
	ID          string
	TopicID     string
	Source      string // "SESSION", "AUTHORED", "AI_ACCEPTED"
	CommitSHA   *string
	FilePath    *string
	CodeSnippet *string
	Confidence  float64
	CreatedAt   time.Time
}

type LessonQueueItem struct {
	TopicID          string
	PriorityScore    float64
	SourceExposureID *string
	QueuedAt         time.Time
}
