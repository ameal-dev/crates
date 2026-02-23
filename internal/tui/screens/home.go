package screens

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/internal/queue"
)

// Home screen aliases for shared palette
var (
	hmSurface   = clrSurface
	hmAccent    = clrAccent
	hmPink      = clrPink
	hmOrange    = clrOrange
	hmGreen     = clrGreen
	hmBlue      = clrBlue
	hmMuted     = clrMuted
	hmFg        = clrFg
	hmDimBorder = clrDimBorder
)

// HomeScreen is the main dashboard shown on startup.
type HomeScreen struct {
	db              *sql.DB
	recentSessions  []models.Session
	topicMap        map[string]models.Topic
	dueRecallCount  int
	progressEntries []models.TopicProgress
	lessonQueue     []models.LessonQueueItem
	queueTopicMap   map[string]models.Topic
	streakDays      int
	selectedIdx     int
	width, height   int
	err             error
}

type homeDataMsg struct {
	sessions      []models.Session
	topicMap      map[string]models.Topic
	dueCount      int
	progress      []models.TopicProgress
	lessonQueue   []models.LessonQueueItem
	queueTopicMap map[string]models.Topic
	streakDays    int
	err           error
}

// NewHomeScreen creates a new home dashboard.
func NewHomeScreen(db *sql.DB) *HomeScreen {
	return &HomeScreen{db: db, topicMap: make(map[string]models.Topic), queueTopicMap: make(map[string]models.Topic)}
}

func (h *HomeScreen) Init() tea.Cmd {
	return h.loadData()
}

func (h *HomeScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "t", "T":
			return h, func() tea.Msg {
				return NavigateMsg{Screen: "topic_browser"}
			}
		case "r", "R":
			return h, func() tea.Msg {
				return NavigateMsg{Screen: "recall"}
			}
		case "p", "P":
			return h, func() tea.Msg {
				return NavigateMsg{Screen: "progress"}
			}
		case "g", "G":
			return h, func() tea.Msg {
				return NavigateMsg{Screen: "knowledge_graph"}
			}
		case "1":
			if len(h.lessonQueue) >= 1 {
				return h, func() tea.Msg {
					return NavigateMsg{Screen: "session", TopicID: h.lessonQueue[0].TopicID}
				}
			}
		case "2":
			if len(h.lessonQueue) >= 2 {
				return h, func() tea.Msg {
					return NavigateMsg{Screen: "session", TopicID: h.lessonQueue[1].TopicID}
				}
			}
		case "3":
			if len(h.lessonQueue) >= 3 {
				return h, func() tea.Msg {
					return NavigateMsg{Screen: "session", TopicID: h.lessonQueue[2].TopicID}
				}
			}
		case "j", "down":
			if h.selectedIdx < len(h.recentSessions)-1 {
				h.selectedIdx++
			}
		case "k", "up":
			if h.selectedIdx > 0 {
				h.selectedIdx--
			}
		case "enter":
			if len(h.recentSessions) > 0 && h.selectedIdx < len(h.recentSessions) {
				sess := h.recentSessions[h.selectedIdx]
				return h, func() tea.Msg {
					return NavigateMsg{Screen: "session", TopicID: sess.TopicID, SessionID: sess.ID}
				}
			}
		}

	case homeDataMsg:
		if msg.err != nil {
			h.err = msg.err
			return h, nil
		}
		h.recentSessions = msg.sessions
		h.topicMap = msg.topicMap
		h.dueRecallCount = msg.dueCount
		h.progressEntries = msg.progress
		h.lessonQueue = msg.lessonQueue
		h.queueTopicMap = msg.queueTopicMap
		h.streakDays = msg.streakDays
		return h, nil

	case NavigateMsg:
		// Bubble up to parent
		return h, func() tea.Msg { return msg }
	}

	return h, nil
}

func (h *HomeScreen) View() string {
	if h.err != nil {
		return fmt.Sprintf("Error loading dashboard: %v", h.err)
	}

	w := h.width
	if w < 40 {
		w = 80
	}

	hero := h.renderHeroStats()
	lessons := h.renderLessons(w, 0)
	sessions := h.renderRecentSessions(w, 0)

	var middle string
	if w >= 80 {
		lessonsW := (w - 3) * 6 / 10
		sessionsW := w - 3 - lessonsW
		left := lipgloss.NewStyle().Width(lessonsW).Render(h.renderLessons(lessonsW, 0))
		right := lipgloss.NewStyle().Width(sessionsW).Render(h.renderRecentSessions(sessionsW, 0))
		middle = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	} else {
		middle = lipgloss.JoinVertical(lipgloss.Left, lessons, sessions)
	}

	nav := h.renderNavBar()

	content := lipgloss.JoinVertical(lipgloss.Left, hero, "", middle, "", nav)

	// Pad the whole thing to fill the terminal height
	lines := strings.Count(content, "\n") + 1
	if h.height > lines {
		content += strings.Repeat("\n", h.height-lines)
	}

	return content
}

func (h *HomeScreen) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *HomeScreen) loadData() tea.Cmd {
	return func() tea.Msg {
		sessions, err := queries.GetRecentSessions(h.db, 5)
		if err != nil {
			return homeDataMsg{err: err}
		}

		// Build topic map for display names
		topicMap := make(map[string]models.Topic)
		for _, s := range sessions {
			if _, ok := topicMap[s.TopicID]; !ok {
				t, err := queries.GetTopic(h.db, s.TopicID)
				if err == nil {
					topicMap[s.TopicID] = t
				}
			}
		}

		dueCount, err := queries.GetDueRecallCardCount(h.db)
		if err != nil {
			return homeDataMsg{err: err}
		}

		progress, err := queries.GetAllTopicProgress(h.db)
		if err != nil {
			return homeDataMsg{err: err}
		}

		// Build daily lesson queue
		lessonQueue, err := queue.BuildDailyQueue(h.db, time.Now())
		if err != nil {
			return homeDataMsg{err: err}
		}

		// Build topic map for queue items
		queueTopicMap := make(map[string]models.Topic)
		for _, item := range lessonQueue {
			if _, ok := queueTopicMap[item.TopicID]; !ok {
				t, err := queries.GetTopic(h.db, item.TopicID)
				if err == nil {
					queueTopicMap[item.TopicID] = t
				}
			}
		}

		// Compute streak
		dates, _ := queries.GetDistinctCompletedSessionDates(h.db, 60)
		streak := computeStreak(dates, time.Now())

		return homeDataMsg{
			sessions:      sessions,
			topicMap:      topicMap,
			dueCount:      dueCount,
			progress:      progress,
			lessonQueue:   lessonQueue,
			queueTopicMap: queueTopicMap,
			streakDays:    streak,
		}
	}
}

func lessonReason(progress []models.TopicProgress, topicID string) string {
	for _, p := range progress {
		if p.TopicID == topicID {
			if p.AIExposureCount > p.AuthoredCount {
				return "AI-generated code — verify understanding"
			}
			if p.LastConfirmedAt != nil && time.Since(*p.LastConfirmedAt) > 14*24*time.Hour {
				return "Confidence has decayed"
			}
			return "Time for a refresher"
		}
	}
	return "New concept detected"
}

// NavigateMsg is used by screens to signal navigation to the root model.
type NavigateMsg struct {
	Screen    string
	TopicID   string
	SessionID string // set when resuming an existing session
}

// --- Helper functions ---

func computeStreak(dates []string, now time.Time) int {
	if len(dates) == 0 {
		return 0
	}

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	streak := 0
	for i, d := range dates {
		if i == 0 {
			// First date must be today or yesterday to have a streak
			if d != today && d != yesterday {
				return 0
			}
			streak = 1
			continue
		}
		// Each subsequent date must be exactly 1 day before the previous
		prev, _ := time.Parse("2006-01-02", dates[i-1])
		curr, _ := time.Parse("2006-01-02", d)
		if prev.Sub(curr).Hours() == 24 {
			streak++
		} else {
			break
		}
	}
	return streak
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1 min"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%d min", m)
}

func statusPill(status string) string {
	switch status {
	case "active":
		return lipgloss.NewStyle().
			Background(hmGreen).
			Foreground(clrDark).
			Padding(0, 1).
			Render("active")
	case "completed":
		return lipgloss.NewStyle().
			Background(hmMuted).
			Foreground(hmFg).
			Padding(0, 1).
			Render("done")
	default:
		return lipgloss.NewStyle().
			Background(hmPink).
			Foreground(clrDark).
			Padding(0, 1).
			Render("left")
	}
}

func difficultyBadge(mastery int) string {
	switch {
	case mastery <= 1:
		return lipgloss.NewStyle().
			Background(hmGreen).
			Foreground(clrDark).
			Padding(0, 1).
			Render("beginner")
	case mastery <= 3:
		return lipgloss.NewStyle().
			Background(hmOrange).
			Foreground(clrDark).
			Padding(0, 1).
			Render("intermediate")
	default:
		return lipgloss.NewStyle().
			Background(hmPink).
			Foreground(clrDark).
			Padding(0, 1).
			Render("advanced")
	}
}

func renderHomeProgressBar(value, max, width int) string {
	if max <= 0 {
		max = 5
	}
	if width <= 0 {
		width = 20
	}
	filled := int(math.Round(float64(value) / float64(max) * float64(width)))
	if filled > width {
		filled = width
	}

	filledStyle := lipgloss.NewStyle().Foreground(hmAccent)
	emptyStyle := lipgloss.NewStyle().Foreground(hmMuted)

	return filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// --- Rendering methods ---

func (h *HomeScreen) renderHeroStats() string {
	w := h.width
	if w < 40 {
		w = 80
	}

	masteryCount := 0
	for _, p := range h.progressEntries {
		if p.MasteryLevel >= 3 {
			masteryCount++
		}
	}

	type stat struct {
		value string
		label string
		color lipgloss.Color
	}
	stats := []stat{
		{fmt.Sprintf("%d", h.dueRecallCount), "Recall Due", hmBlue},
		{fmt.Sprintf("%d", h.streakDays), "Day Streak", hmOrange},
		{fmt.Sprintf("%d", masteryCount), "Mastered", hmGreen},
	}

	cardWidth := (w - 6) / 3
	if cardWidth < 12 {
		cardWidth = 12
	}

	var cards []string
	for _, s := range stats {
		valueStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(s.color).
			Width(cardWidth - 4).
			Align(lipgloss.Center)

		labelStyle := lipgloss.NewStyle().
			Foreground(hmMuted).
			Width(cardWidth - 4).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(lipgloss.Center,
			valueStyle.Render(s.value),
			labelStyle.Render(s.label),
		)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(hmDimBorder).
			Background(hmSurface).
			Width(cardWidth).
			Padding(1, 0).
			Align(lipgloss.Center).
			Render(content)

		cards = append(cards, card)
	}

	if w < 50 {
		return lipgloss.JoinVertical(lipgloss.Left, cards...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (h *HomeScreen) renderLessons(width, _ int) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(hmFg).
		MarginBottom(1)

	header := headerStyle.Render("  Today's Lessons")

	if len(h.lessonQueue) == 0 {
		var msg string
		if len(h.progressEntries) == 0 {
			msg = "Start a session to get personalized lessons."
		} else {
			msg = "No lessons queued — you're on track."
		}
		emptyStyle := lipgloss.NewStyle().
			Foreground(hmMuted).
			Padding(1, 2)
		return lipgloss.JoinVertical(lipgloss.Left, header, emptyStyle.Render(msg))
	}

	// Build progress map for badge lookup
	progressMap := make(map[string]models.TopicProgress)
	for _, p := range h.progressEntries {
		progressMap[p.TopicID] = p
	}

	var cards []string
	for i, item := range h.lessonQueue {
		cards = append(cards, h.renderLessonCard(i, item, progressMap, width))
	}

	parts := []string{header}
	parts = append(parts, cards...)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (h *HomeScreen) renderLessonCard(idx int, item models.LessonQueueItem, progressMap map[string]models.TopicProgress, width int) string {
	topicTitle := item.TopicID
	if t, ok := h.queueTopicMap[item.TopicID]; ok {
		topicTitle = t.Title
	}

	mastery := 0
	if p, ok := progressMap[item.TopicID]; ok {
		mastery = p.MasteryLevel
	}

	cardWidth := width - 4
	if cardWidth < 20 {
		cardWidth = 20
	}

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(hmFg)
	mutedStyle := lipgloss.NewStyle().Foreground(hmMuted)
	keyStyle := lipgloss.NewStyle().Foreground(hmAccent).Bold(true)

	// Top line: number + name + badge
	topLine := fmt.Sprintf("%s  %s",
		nameStyle.Render(fmt.Sprintf("%d. %s", idx+1, topicTitle)),
		difficultyBadge(mastery),
	)

	// Reason
	reason := mutedStyle.Render(lessonReason(h.progressEntries, item.TopicID))

	// Progress bar
	barWidth := cardWidth - 4
	if barWidth > 30 {
		barWidth = 30
	}
	if barWidth < 8 {
		barWidth = 8
	}
	bar := renderHomeProgressBar(mastery, 5, barWidth)

	// Key hint
	hint := keyStyle.Render(fmt.Sprintf("[%d]", idx+1)) + mutedStyle.Render(" Start")

	content := lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		reason,
		bar,
		hint,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(hmDimBorder).
		Background(hmSurface).
		Width(cardWidth).
		Padding(0, 1).
		MarginBottom(1).
		Render(content)
}

func (h *HomeScreen) renderRecentSessions(width, _ int) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(hmFg).
		MarginBottom(1)

	header := headerStyle.Render("  Recent Sessions")

	if len(h.recentSessions) == 0 {
		keyStyle := lipgloss.NewStyle().Foreground(hmAccent).Bold(true)
		emptyStyle := lipgloss.NewStyle().
			Foreground(hmMuted).
			Padding(1, 2)
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			emptyStyle.Render("No sessions yet. Press "+keyStyle.Render("t")+" to start learning."),
		)
	}

	cursorStyle := lipgloss.NewStyle().Foreground(hmAccent)
	nameStyle := lipgloss.NewStyle().Foreground(hmFg)
	mutedStyle := lipgloss.NewStyle().Foreground(hmMuted)

	maxNameLen := width - 36
	if maxNameLen < 10 {
		maxNameLen = 10
	}

	var rows []string
	for i, sess := range h.recentSessions {
		cursor := "  "
		if i == h.selectedIdx {
			cursor = cursorStyle.Render("▸ ")
		}

		topicTitle := sess.TopicID
		if t, ok := h.topicMap[sess.TopicID]; ok {
			topicTitle = t.Title
		}
		topicTitle = truncate(topicTitle, maxNameLen)

		pill := statusPill(sess.Status)
		rel := mutedStyle.Render(relativeTime(sess.LastActivityAt))
		dur := mutedStyle.Render(formatDuration(sess.LastActivityAt.Sub(sess.StartedAt)))

		row := fmt.Sprintf("%s%s  %s  %s  %s",
			cursor,
			nameStyle.Render(topicTitle),
			pill,
			rel,
			dur,
		)
		rows = append(rows, row)
	}

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(hmDimBorder).
		Background(hmSurface).
		Width(width - 4).
		Padding(1, 1).
		Render(strings.Join(rows, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (h *HomeScreen) renderNavBar() string {
	w := h.width
	if w < 40 {
		w = 80
	}

	keyStyle := lipgloss.NewStyle().Foreground(hmAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(hmMuted)
	sepStyle := lipgloss.NewStyle().Foreground(hmDimBorder)

	sep := sepStyle.Render("  |  ")

	items := []string{
		keyStyle.Render("[t]") + descStyle.Render(" Topics"),
		keyStyle.Render("[g]") + descStyle.Render(" Knowledge"),
		keyStyle.Render("[r]") + descStyle.Render(" Recall"),
		keyStyle.Render("[p]") + descStyle.Render(" Progress"),
		keyStyle.Render("[ctrl+c]") + descStyle.Render(" Quit"),
	}

	bar := strings.Join(items, sep)

	borderStyle := lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(hmDimBorder).
		Width(w - 2).
		Padding(0, 1).
		Align(lipgloss.Center)

	return borderStyle.Render(bar)
}
