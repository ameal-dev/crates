package screens

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

// Progress screen aliases for shared palette
var (
	pgSurface   = clrSurface
	pgAccent    = clrAccent
	pgOrange    = clrOrange
	pgGreen     = clrGreen
	pgBlue      = clrBlue
	pgMuted     = clrMuted
	pgFg        = clrFg
	pgDimBorder = clrDimBorder
)

// ProgressScreen shows per-topic mastery and learning stats.
type ProgressScreen struct {
	db          *sql.DB
	topics      []models.Topic
	progressMap map[string]models.TopicProgress
	width       int
	height      int
	err         error
}

type progressDataMsg struct {
	topics   []models.Topic
	progress []models.TopicProgress
	err      error
}

// NewProgressScreen creates a new progress overview screen.
func NewProgressScreen(db *sql.DB) *ProgressScreen {
	return &ProgressScreen{db: db, progressMap: make(map[string]models.TopicProgress)}
}

func (p *ProgressScreen) Init() tea.Cmd {
	return p.loadData()
}

func (p *ProgressScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case progressDataMsg:
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.topics = msg.topics
		for _, pr := range msg.progress {
			p.progressMap[pr.TopicID] = pr
		}
		return p, nil
	}
	return p, nil
}

func (p *ProgressScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(pgFg).
		MarginBottom(1)

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Progress"))
	b.WriteString("\n\n")

	if p.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(pgMuted)
		b.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", p.err)))
		b.WriteString("\n")
		return b.String()
	}

	if len(p.topics) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(pgMuted)
		b.WriteString(emptyStyle.Render("  No topics loaded yet."))
		b.WriteString("\n")
		return b.String()
	}

	// Hero stats
	b.WriteString(p.renderProgressHeroStats())
	b.WriteString("\n\n")

	// Topic groups
	barWidth := min(30, p.width/3)
	if barWidth < 5 {
		barWidth = 10
	}

	nameStyle := lipgloss.NewStyle().Foreground(pgFg)
	rootStyle := lipgloss.NewStyle().Bold(true).Foreground(pgAccent)
	scoreStyle := lipgloss.NewStyle().Foreground(pgAccent)
	hintStyle := lipgloss.NewStyle().Foreground(pgMuted)

	for _, topic := range p.topics {
		b.WriteString("  " + rootStyle.Render(topic.Title))
		b.WriteString("\n\n")

		for _, child := range topic.Children {
			prog := p.progressMap[child.ID]
			bar := pgRenderProgressBar(prog.MasteryLevel, 5, barWidth, prog.HintsUsed, prog.Skips)
			score := scoreStyle.Render(fmt.Sprintf("%d/5", prog.MasteryLevel))

			line := fmt.Sprintf("  %s  %s  %s", nameStyle.Render(fmt.Sprintf("%-20s", child.Title)), bar, score)
			if prog.HintsUsed > 0 || prog.Skips > 0 {
				line += "  " + hintStyle.Render(fmt.Sprintf("hints:%d skips:%d", prog.HintsUsed, prog.Skips))
			}

			// Bordered card for categories
			cardStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(pgDimBorder).
				Background(pgSurface).
				Padding(0, 1)
			if p.width > 10 {
				cardStyle = cardStyle.Width(p.width - 6)
			}
			b.WriteString("  " + cardStyle.Render(line))
			b.WriteString("\n")

			for _, leaf := range child.Children {
				leafProg := p.progressMap[leaf.ID]
				leafBar := pgRenderProgressBar(leafProg.MasteryLevel, 5, barWidth, leafProg.HintsUsed, leafProg.Skips)
				leafScore := scoreStyle.Render(fmt.Sprintf("%d/5", leafProg.MasteryLevel))
				b.WriteString(fmt.Sprintf("      %s  %s  %s\n",
					nameStyle.Render(fmt.Sprintf("%-18s", leaf.Title)),
					leafBar,
					leafScore,
				))
			}
		}
		b.WriteString("\n")
	}

	// Nav bar
	b.WriteString(p.renderProgressNavBar())

	return b.String()
}

func (p *ProgressScreen) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *ProgressScreen) loadData() tea.Cmd {
	return func() tea.Msg {
		topics, err := queries.GetTopicTree(p.db)
		if err != nil {
			return progressDataMsg{err: err}
		}
		progress, err := queries.GetAllTopicProgress(p.db)
		if err != nil {
			return progressDataMsg{err: err}
		}
		return progressDataMsg{topics: topics, progress: progress}
	}
}

func (p *ProgressScreen) renderProgressHeroStats() string {
	w := p.width
	if w < 40 {
		w = 80
	}

	// Count leaf topics by walking 3-level tree
	var mastered, inProgress, notStarted int
	for _, root := range p.topics {
		for _, cat := range root.Children {
			leaves := cat.Children
			if len(leaves) == 0 {
				// Category with no children counts as a leaf itself
				leaves = []models.Topic{cat}
			}
			for _, leaf := range leaves {
				prog := p.progressMap[leaf.ID]
				switch {
				case prog.MasteryLevel >= 3:
					mastered++
				case prog.MasteryLevel >= 1:
					inProgress++
				default:
					notStarted++
				}
			}
		}
	}

	type stat struct {
		value string
		label string
		color lipgloss.Color
	}
	stats := []stat{
		{fmt.Sprintf("%d", mastered), "Mastered", pgGreen},
		{fmt.Sprintf("%d", inProgress), "In Progress", pgOrange},
		{fmt.Sprintf("%d", notStarted), "Not Started", pgMuted},
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
			Foreground(pgMuted).
			Width(cardWidth - 4).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(lipgloss.Center,
			valueStyle.Render(s.value),
			labelStyle.Render(s.label),
		)

		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(pgDimBorder).
			Background(pgSurface).
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

func pgRenderProgressBar(value, max, width, hints, skips int) string {
	if width < 5 {
		width = 5
	}
	filled := (value * width) / max
	empty := width - filled

	var fillColor lipgloss.Color
	switch {
	case value == 0:
		fillColor = pgMuted
	case hints > 0 || skips > 0:
		fillColor = pgOrange
	default:
		fillColor = pgGreen
	}

	filledStyle := lipgloss.NewStyle().Foreground(fillColor)
	emptyStyle := lipgloss.NewStyle().Foreground(pgMuted)

	return filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
}

func (p *ProgressScreen) renderProgressNavBar() string {
	w := p.width
	if w < 40 {
		w = 80
	}

	keyStyle := lipgloss.NewStyle().Foreground(pgAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(pgMuted)
	sepStyle := lipgloss.NewStyle().Foreground(pgDimBorder)

	sep := sepStyle.Render("  |  ")

	items := []string{
		keyStyle.Render("[ctrl+h]") + descStyle.Render(" Home"),
		keyStyle.Render("[ctrl+c]") + descStyle.Render(" Quit"),
	}

	bar := strings.Join(items, sep)

	borderStyle := lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(pgDimBorder).
		Width(w - 2).
		Padding(0, 1).
		Align(lipgloss.Center)

	return borderStyle.Render(bar)
}
