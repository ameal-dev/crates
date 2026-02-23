package screens

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/confidence"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/internal/spaced"
)

// RecallScreen handles spaced repetition card review.
type RecallScreen struct {
	db             *sql.DB
	aiClient       ai.Streamer
	cards          []models.RecallCard
	current        int
	revealed       bool
	done           bool
	totalCardCount int
	width          int
	height         int
	err            error
}

type recallDataMsg struct {
	cards      []models.RecallCard
	totalCount int
	err        error
}

// NewRecallScreen creates a new recall review screen.
func NewRecallScreen(db *sql.DB, aiClient ai.Streamer) *RecallScreen {
	return &RecallScreen{db: db, aiClient: aiClient}
}

func (r *RecallScreen) Init() tea.Cmd {
	return r.loadCards()
}

func (r *RecallScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if !r.revealed && !r.done {
				r.revealed = true
			}
		case "1", "2", "3", "4", "5":
			if r.revealed && !r.done {
				quality, _ := strconv.Atoi(msg.String())
				card := r.cards[r.current]
				result := spaced.SM2(quality, card.Repetitions, card.EaseFactor, card.IntervalDays, time.Now())
				_ = queries.UpdateRecallCard(r.db, card.ID, result.NextReview, result.IntervalDays, result.EaseFactor, result.Repetitions)
				if prog, err := queries.GetTopicProgress(r.db, card.TopicID); err == nil {
					prog = confidence.UpdateAfterRecall(prog, quality)
					_ = queries.UpsertTopicProgress(r.db, prog)
				}
				r.revealed = false
				r.current++
				if r.current >= len(r.cards) {
					r.done = true
				}
			}
		}

	case recallDataMsg:
		if msg.err != nil {
			r.err = msg.err
			return r, nil
		}
		r.cards = msg.cards
		r.totalCardCount = msg.totalCount
		r.current = 0
		r.revealed = false
		r.done = len(msg.cards) == 0
		return r, nil
	}

	return r, nil
}

func (r *RecallScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		MarginBottom(1)

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4B5563")).
		Padding(1, 2).
		Width(min(60, r.width-4))

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Recall"))
	b.WriteString("\n\n")

	if r.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", r.err))
		return b.String()
	}

	if r.done || len(r.cards) == 0 {
		if r.totalCardCount == 0 {
			b.WriteString("  No recall cards yet. Cards are created as you complete lessons.\n\n")
		} else {
			b.WriteString("  All caught up! No cards due for review.\n\n")
		}
		b.WriteString(subtleStyle.Render("  Press ctrl+h to go home."))
		return b.String()
	}

	card := r.cards[r.current]
	b.WriteString(fmt.Sprintf("  Card %d of %d\n\n", r.current+1, len(r.cards)))

	// Question
	b.WriteString(cardStyle.Render(card.Question))
	b.WriteString("\n\n")

	if r.revealed {
		answerStyle := cardStyle.BorderForeground(lipgloss.Color("#10B981"))
		b.WriteString(answerStyle.Render(card.Answer))
		b.WriteString("\n\n")
		b.WriteString(subtleStyle.Render("  Rate: 1=forgot 2=hard 3=ok 4=good 5=easy"))
	} else {
		b.WriteString(subtleStyle.Render("  Press enter or space to reveal answer"))
	}

	return b.String()
}

func (r *RecallScreen) SetSize(width, height int) {
	r.width = width
	r.height = height
}

func (r *RecallScreen) loadCards() tea.Cmd {
	return func() tea.Msg {
		cards, err := queries.GetDueRecallCards(r.db, 20)
		if err != nil {
			return recallDataMsg{err: err}
		}
		totalCount, err := queries.GetTotalRecallCardCount(r.db)
		if err != nil {
			return recallDataMsg{err: err}
		}
		return recallDataMsg{cards: cards, totalCount: totalCount}
	}
}
