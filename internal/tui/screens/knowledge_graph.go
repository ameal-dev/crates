package screens

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/confidence"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

// Cell rendering constants.
const (
	cellSolid     = "██"
	cellGood      = "▓▓"
	cellShaky     = "▒▒"
	cellWeak      = "░░"
	cellUntouched = "  "
)

// Color constants.
var (
	colorGreen   = lipgloss.Color("#10B981")
	colorTeal    = lipgloss.Color("#06B6D4")
	colorYellow  = lipgloss.Color("#EAB308")
	colorAmber   = lipgloss.Color("#F59E0B")
	colorRed     = lipgloss.Color("#EF4444")
	colorDimGray = lipgloss.Color("#374151")
)

// Tokyo Night palette for knowledge graph chrome.
var (
	kgSurface   = lipgloss.Color("#1e2030")
	kgAccent    = lipgloss.Color("#bb9af7")
	kgOrange    = lipgloss.Color("#e0af68")
	kgGreen     = lipgloss.Color("#9ece6a")
	kgBlue      = lipgloss.Color("#7aa2f7")
	kgMuted     = lipgloss.Color("#565f89")
	kgFg        = lipgloss.Color("#c0caf5")
	kgDimBorder = lipgloss.Color("#3b3d57")
)

// graphTopicNode holds the view model data for a single leaf topic cell.
type graphTopicNode struct {
	topicID          string
	title            string
	parentTitle      string
	languageTitle    string
	masteryLevel     int
	confidenceMod    float64
	effectiveMastery float64
	aiExposureCount  int
	authoredCount    int
	sessionCount     int
	lastConfirmedAt  *time.Time
	dueRecallCount   int
	totalRecallCount int
	inLessonQueue    bool
	hasProgress      bool
}

type graphCategory struct {
	title string
	nodes []graphTopicNode
}

type graphLanguage struct {
	title      string
	categories []graphCategory
}

// KnowledgeGraphScreen displays a heatmap of the user's knowledge state.
type KnowledgeGraphScreen struct {
	db            *sql.DB
	languages     []graphLanguage
	flatCells     []graphTopicNode
	selectedIdx   int
	scrollOffset  int
	width, height int
	err           error
}

type graphDataMsg struct {
	languages []graphLanguage
	flatCells []graphTopicNode
	err       error
}

// NewKnowledgeGraphScreen creates a new knowledge graph screen.
func NewKnowledgeGraphScreen(db *sql.DB) *KnowledgeGraphScreen {
	return &KnowledgeGraphScreen{db: db}
}

func (g *KnowledgeGraphScreen) Init() tea.Cmd {
	return g.loadData()
}

func (g *KnowledgeGraphScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if g.selectedIdx < len(g.flatCells)-1 {
				g.selectedIdx++
				g.adjustScroll()
			}
		case "k", "up":
			if g.selectedIdx > 0 {
				g.selectedIdx--
				g.adjustScroll()
			}
		case "enter":
			if len(g.flatCells) > 0 && g.selectedIdx < len(g.flatCells) {
				topicID := g.flatCells[g.selectedIdx].topicID
				return g, func() tea.Msg {
					return NavigateMsg{Screen: "session", TopicID: topicID}
				}
			}
		case "r", "R":
			return g, func() tea.Msg {
				return NavigateMsg{Screen: "recall"}
			}
		}

	case graphDataMsg:
		if msg.err != nil {
			g.err = msg.err
			return g, nil
		}
		g.languages = msg.languages
		g.flatCells = msg.flatCells
		g.selectedIdx = 0
		g.scrollOffset = 0
		return g, nil

	case NavigateMsg:
		return g, func() tea.Msg { return msg }
	}

	return g, nil
}

func (g *KnowledgeGraphScreen) View() string {
	if g.err != nil {
		return fmt.Sprintf("Error loading knowledge graph: %v", g.err)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(kgFg).
		MarginBottom(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(kgAccent).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().
		Foreground(kgMuted)

	var b strings.Builder

	// Title bar
	b.WriteString(titleStyle.Render("  Knowledge Graph"))
	b.WriteString("\n\n")

	// Summary banner
	b.WriteString(g.renderSummary())
	b.WriteString("\n\n")

	// Empty state
	if len(g.flatCells) == 0 {
		b.WriteString(mutedStyle.Render("  Start a session from the topic browser to see your knowledge map take shape."))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s browse topics  •  %s home  •  %s quit\n",
			keyStyle.Render("[t]"),
			keyStyle.Render("[ctrl+h]"),
			keyStyle.Render("[ctrl+c]"),
		))
		return b.String()
	}

	wideLayout := g.width >= 90

	if wideLayout {
		heatmap := g.renderHeatmap()
		sidebar := g.renderSidebar()
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, heatmap, "  ", sidebar))
	} else {
		b.WriteString(g.renderHeatmap())
		b.WriteString("\n")
		b.WriteString(g.renderCompactDetail())
	}

	b.WriteString("\n")

	// Legend
	b.WriteString(g.renderLegend())
	b.WriteString("\n\n")

	// Navigation bar
	b.WriteString(g.renderKgNavBar())

	return b.String()
}

func (g *KnowledgeGraphScreen) SetSize(width, height int) {
	g.width = width
	g.height = height
}

// adjustScroll keeps the selected cell visible within the heatmap area.
func (g *KnowledgeGraphScreen) adjustScroll() {
	// Approximate visible rows (reserve space for header/footer)
	visibleRows := g.height - 12
	if visibleRows < 3 {
		visibleRows = 3
	}

	// Find which row the selected cell is on
	row := g.rowForIndex(g.selectedIdx)

	if row < g.scrollOffset {
		g.scrollOffset = row
	}
	if row >= g.scrollOffset+visibleRows {
		g.scrollOffset = row - visibleRows + 1
	}
}

// rowForIndex returns the heatmap row index for a given flatCells index.
func (g *KnowledgeGraphScreen) rowForIndex(idx int) int {
	row := 0
	cellIdx := 0
	for _, lang := range g.languages {
		row++ // language header row
		for _, cat := range lang.categories {
			if cellIdx+len(cat.nodes) > idx {
				return row
			}
			cellIdx += len(cat.nodes)
			row++
		}
	}
	return row
}

func (g *KnowledgeGraphScreen) renderSummary() string {
	numStyle := lipgloss.NewStyle().Foreground(kgBlue).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(kgMuted)

	total := len(g.flatCells)
	var solid, good, shaky, weak, untouched int
	var totalConfidence float64
	var confidenceCount int
	var lessonsQueued, recallDue int

	for _, node := range g.flatCells {
		switch {
		case !node.hasProgress:
			untouched++
		case node.effectiveMastery > 3.0:
			solid++
		case node.effectiveMastery >= 2.0:
			good++
		case node.effectiveMastery >= 1.0:
			shaky++
		default:
			weak++
		}
		if node.hasProgress {
			totalConfidence += node.confidenceMod
			confidenceCount++
		}
		if node.inLessonQueue {
			lessonsQueued++
		}
		recallDue += node.dueRecallCount
	}

	avgConf := 0.0
	if confidenceCount > 0 {
		avgConf = totalConfidence / float64(confidenceCount)
	}

	return fmt.Sprintf("  %s %s | %s %s  %s %s  %s %s  %s %s  %s %s\n  %s %s | %s %s | %s %s",
		numStyle.Render(fmt.Sprintf("%d", total)), labelStyle.Render("topics"),
		numStyle.Render(fmt.Sprintf("%d", solid)), labelStyle.Render("solid"),
		numStyle.Render(fmt.Sprintf("%d", good)), labelStyle.Render("good"),
		numStyle.Render(fmt.Sprintf("%d", shaky)), labelStyle.Render("shaky"),
		numStyle.Render(fmt.Sprintf("%d", weak)), labelStyle.Render("weak"),
		numStyle.Render(fmt.Sprintf("%d", untouched)), labelStyle.Render("untouched"),
		labelStyle.Render("Avg confidence:"), numStyle.Render(fmt.Sprintf("%.2f", avgConf)),
		numStyle.Render(fmt.Sprintf("%d", lessonsQueued)), labelStyle.Render("lessons queued"),
		numStyle.Render(fmt.Sprintf("%d", recallDue)), labelStyle.Render("recall due"),
	)
}

func (g *KnowledgeGraphScreen) renderHeatmap() string {
	langStyle := lipgloss.NewStyle().Bold(true).Foreground(kgAccent)
	catStyle := lipgloss.NewStyle().Foreground(kgMuted).Width(14).Align(lipgloss.Right)
	selectedStyle := lipgloss.NewStyle().Reverse(true)

	var b strings.Builder
	cellIdx := 0

	for _, lang := range g.languages {
		b.WriteString("  " + langStyle.Render(lang.title))
		b.WriteString("\n")

		for _, cat := range lang.categories {
			b.WriteString("  " + catStyle.Render(truncate(cat.title, 14)) + "  ")

			for _, node := range cat.nodes {
				cell := renderCell(node)
				if cellIdx == g.selectedIdx {
					cell = selectedStyle.Render("[" + cellText(node) + "]")
				}
				b.WriteString(cell + " ")
				cellIdx++
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (g *KnowledgeGraphScreen) renderLegend() string {
	labelStyle := lipgloss.NewStyle().Foreground(kgMuted)
	pillDark := lipgloss.Color("#1a1b26")

	pill := func(text string, bg lipgloss.Color) string {
		return lipgloss.NewStyle().
			Background(bg).
			Foreground(pillDark).
			Padding(0, 1).
			Render(text)
	}

	return fmt.Sprintf("  %s  %s %s  %s %s  %s %s  %s %s",
		labelStyle.Render("Legend"),
		pill(cellSolid, colorGreen), labelStyle.Render(">3"),
		pill(cellGood, colorTeal), labelStyle.Render("2-3"),
		pill(cellShaky, colorYellow), labelStyle.Render("1-2"),
		pill(cellWeak, colorRed), labelStyle.Render("<1"),
	)
}

func (g *KnowledgeGraphScreen) renderSidebar() string {
	if len(g.flatCells) == 0 || g.selectedIdx >= len(g.flatCells) {
		return ""
	}

	node := g.flatCells[g.selectedIdx]
	width := 28
	pillDark := lipgloss.Color("#1a1b26")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(kgDimBorder).
		Background(kgSurface).
		Padding(0, 1).
		Width(width)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(kgAccent)
	labelStyle := lipgloss.NewStyle().Foreground(kgMuted)
	valueStyle := lipgloss.NewStyle().Foreground(kgFg)
	keyStyle := lipgloss.NewStyle().Foreground(kgAccent).Bold(true)

	var b strings.Builder

	b.WriteString(titleStyle.Render(truncate(node.title, width-4)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Mastery:") + "     " + valueStyle.Render(fmt.Sprintf("%d/5", node.masteryLevel)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Eff.mastery:") + " " + valueStyle.Render(fmt.Sprintf("%.1f", node.effectiveMastery)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Confidence:") + "  " + valueStyle.Render(fmt.Sprintf("%.2f", node.confidenceMod)))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("AI accepted:") + " " + valueStyle.Render(fmt.Sprintf("%d", node.aiExposureCount)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Authored:") + "    " + valueStyle.Render(fmt.Sprintf("%d", node.authoredCount)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Sessions:") + "    " + valueStyle.Render(fmt.Sprintf("%d", node.sessionCount)))
	b.WriteString("\n")

	if node.lastConfirmedAt != nil {
		days := int(time.Since(*node.lastConfirmedAt).Hours() / 24)
		var ago string
		if days == 0 {
			ago = "today"
		} else if days == 1 {
			ago = "1 day ago"
		} else {
			ago = fmt.Sprintf("%d days ago", days)
		}
		b.WriteString(labelStyle.Render("Last confirmed:"))
		b.WriteString("\n")
		b.WriteString("  " + valueStyle.Render(ago))
	} else {
		b.WriteString(labelStyle.Render("Last confirmed:"))
		b.WriteString("\n")
		b.WriteString("  " + valueStyle.Render("never"))
	}
	b.WriteString("\n\n")

	// Diagnostic
	diag := confidenceDiagnostic(node)
	if diag != "" {
		diagPill := lipgloss.NewStyle().
			Background(kgOrange).
			Foreground(pillDark).
			Padding(0, 1).
			Render(diag)
		b.WriteString(diagPill)
		b.WriteString("\n\n")
	}

	// Recall info
	if node.totalRecallCount > 0 {
		recallInfo := fmt.Sprintf("Recall: %d cards", node.totalRecallCount)
		if node.dueRecallCount > 0 {
			recallInfo += fmt.Sprintf(" (%d due)", node.dueRecallCount)
		}
		b.WriteString(labelStyle.Render(recallInfo))
		b.WriteString("\n")
	}

	if node.inLessonQueue {
		queuePill := lipgloss.NewStyle().
			Background(kgBlue).
			Foreground(pillDark).
			Padding(0, 1).
			Render("In lesson queue")
		b.WriteString(queuePill)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(keyStyle.Render("[enter]") + " study  " + keyStyle.Render("[r]") + " recall cards")

	return borderStyle.Render(b.String())
}

func (g *KnowledgeGraphScreen) renderCompactDetail() string {
	if len(g.flatCells) == 0 || g.selectedIdx >= len(g.flatCells) {
		return ""
	}

	node := g.flatCells[g.selectedIdx]
	labelStyle := lipgloss.NewStyle().Foreground(kgMuted)
	accentStyle := lipgloss.NewStyle().Foreground(kgAccent)
	warnStyle := lipgloss.NewStyle().Foreground(kgOrange)

	detail := fmt.Sprintf("  %s  mastery:%s eff:%s conf:%s",
		accentStyle.Render(node.title),
		labelStyle.Render(fmt.Sprintf("%d/5", node.masteryLevel)),
		labelStyle.Render(fmt.Sprintf("%.1f", node.effectiveMastery)),
		labelStyle.Render(fmt.Sprintf("%.2f", node.confidenceMod)),
	)

	diag := confidenceDiagnostic(node)
	if diag != "" {
		detail += "  " + warnStyle.Render(diag)
	}

	return detail
}

func (g *KnowledgeGraphScreen) renderKgNavBar() string {
	w := g.width
	if w < 40 {
		w = 80
	}

	keyStyle := lipgloss.NewStyle().Foreground(kgAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(kgMuted)
	sepStyle := lipgloss.NewStyle().Foreground(kgDimBorder)

	sep := sepStyle.Render("  |  ")

	items := []string{
		keyStyle.Render("[j/k]") + descStyle.Render(" Navigate"),
		keyStyle.Render("[enter]") + descStyle.Render(" Study"),
		keyStyle.Render("[r]") + descStyle.Render(" Recall"),
		keyStyle.Render("[ctrl+h]") + descStyle.Render(" Home"),
	}

	bar := strings.Join(items, sep)

	borderStyle := lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(kgDimBorder).
		Width(w - 2).
		Padding(0, 1).
		Align(lipgloss.Center)

	return borderStyle.Render(bar)
}

// confidenceDiagnostic returns a human-readable explanation of why confidence
// is at its current level for the given topic node.
func confidenceDiagnostic(node graphTopicNode) string {
	if !node.hasProgress {
		return "Needs focused study."
	}

	if node.confidenceMod >= 0.9 && node.masteryLevel >= 3 {
		return "Solid. Confirmed understanding."
	}

	if node.confidenceMod < 0.5 && node.aiExposureCount > node.authoredCount*3 {
		return "Confidence eroding: high AI exposure, minimal self-authored code."
	}

	if node.confidenceMod < 0.7 && node.lastConfirmedAt != nil {
		weeksSince := time.Since(*node.lastConfirmedAt).Hours() / (24 * 7)
		if weeksSince > 4 {
			return fmt.Sprintf("Confidence decaying: last confirmed %d weeks ago.", int(weeksSince))
		}
	}

	if node.masteryLevel >= 3 && node.confidenceMod < 0.7 {
		return "High mastery but low confidence. Consider a session."
	}

	if node.effectiveMastery < 1.0 {
		return "Needs focused study."
	}

	return ""
}

// renderCell returns a styled 2-char cell for a topic node.
func renderCell(node graphTopicNode) string {
	text := cellText(node)
	color := cellColor(node)
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

// cellText returns the raw 2-char text for a cell.
func cellText(node graphTopicNode) string {
	if !node.hasProgress {
		return cellUntouched
	}

	// Amber variant: high mastery but low confidence
	if node.masteryLevel >= 3 && node.confidenceMod < 0.7 {
		return cellShaky
	}

	switch {
	case node.effectiveMastery > 3.0:
		return cellSolid
	case node.effectiveMastery >= 2.0:
		return cellGood
	case node.effectiveMastery >= 1.0:
		return cellShaky
	default:
		return cellWeak
	}
}

// cellColor returns the color for a cell based on effective mastery.
func cellColor(node graphTopicNode) lipgloss.Color {
	if !node.hasProgress {
		return colorDimGray
	}

	// Amber variant: high mastery but low confidence
	if node.masteryLevel >= 3 && node.confidenceMod < 0.7 {
		return colorAmber
	}

	switch {
	case node.effectiveMastery > 3.0:
		return colorGreen
	case node.effectiveMastery >= 2.0:
		return colorTeal
	case node.effectiveMastery >= 1.0:
		return colorYellow
	default:
		return colorRed
	}
}

func (g *KnowledgeGraphScreen) loadData() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()

		allProgress, err := queries.GetAllTopicProgress(g.db)
		if err != nil {
			return graphDataMsg{err: err}
		}

		sessionCounts, err := queries.GetSessionCountByTopic(g.db)
		if err != nil {
			return graphDataMsg{err: err}
		}

		recallTotal, recallDue, err := queries.GetRecallCardCountsByTopic(g.db)
		if err != nil {
			return graphDataMsg{err: err}
		}

		lessonQueue, err := queries.GetLessonQueue(g.db, 3)
		if err != nil {
			return graphDataMsg{err: err}
		}

		// Build lookup maps
		progressMap := make(map[string]models.TopicProgress, len(allProgress))
		for _, p := range allProgress {
			progressMap[p.TopicID] = p
		}

		lessonSet := make(map[string]bool, len(lessonQueue))
		for _, item := range lessonQueue {
			lessonSet[item.TopicID] = true
		}

		// Build a proper 3-level tree from flat topic query.
		// GetTopicTree loses grandchildren due to value copies, so we
		// build it ourselves with pointer-based child assignment.
		languages, flatCells, err := buildTopicGraph(g.db, progressMap, sessionCounts, recallTotal, recallDue, lessonSet, now)
		if err != nil {
			return graphDataMsg{err: err}
		}

		return graphDataMsg{
			languages: languages,
			flatCells: flatCells,
		}
	}
}

// buildTopicGraph queries all topics flat and constructs a proper 3-level tree.
func buildTopicGraph(
	db *sql.DB,
	progressMap map[string]models.TopicProgress,
	sessionCounts map[string]int,
	recallTotal, recallDue map[string]int,
	lessonSet map[string]bool,
	now time.Time,
) ([]graphLanguage, []graphTopicNode, error) {
	// Query all topics flat
	rows, err := db.Query(`SELECT id, parent_id, slug, title, description, sort_order FROM topics ORDER BY sort_order`)
	if err != nil {
		return nil, nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	type topicEntry struct {
		id       string
		parentID *string
		title    string
		children []*topicEntry
	}

	var all []*topicEntry
	byID := make(map[string]*topicEntry)
	for rows.Next() {
		var id, slug, title, desc string
		var parentID *string
		var sortOrder int
		if err := rows.Scan(&id, &parentID, &slug, &title, &desc, &sortOrder); err != nil {
			return nil, nil, fmt.Errorf("scan topic: %w", err)
		}
		e := &topicEntry{id: id, parentID: parentID, title: title}
		all = append(all, e)
		byID[id] = e
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Build tree via pointers
	var roots []*topicEntry
	for _, e := range all {
		if e.parentID == nil {
			roots = append(roots, e)
		} else if parent, ok := byID[*e.parentID]; ok {
			parent.children = append(parent.children, e)
		}
	}

	// Walk 3-level tree: roots → categories → leaves
	var languages []graphLanguage
	var flatCells []graphTopicNode

	for _, root := range roots {
		lang := graphLanguage{title: root.title}

		for _, cat := range root.children {
			category := graphCategory{title: cat.title}

			for _, leaf := range cat.children {
				topic := models.Topic{ID: leaf.id, Title: leaf.title}
				node := buildGraphNode(topic, cat.title, root.title, progressMap, sessionCounts, recallTotal, recallDue, lessonSet, now)
				category.nodes = append(category.nodes, node)
				flatCells = append(flatCells, node)
			}

			// If a category has no children, treat it as a leaf itself
			if len(cat.children) == 0 {
				topic := models.Topic{ID: cat.id, Title: cat.title}
				node := buildGraphNode(topic, root.title, root.title, progressMap, sessionCounts, recallTotal, recallDue, lessonSet, now)
				category.nodes = append(category.nodes, node)
				flatCells = append(flatCells, node)
			}

			if len(category.nodes) > 0 {
				lang.categories = append(lang.categories, category)
			}
		}

		// If a root has no subcategories, treat its children as leaves
		if len(lang.categories) == 0 && len(root.children) > 0 {
			category := graphCategory{title: root.title}
			for _, leaf := range root.children {
				topic := models.Topic{ID: leaf.id, Title: leaf.title}
				node := buildGraphNode(topic, root.title, root.title, progressMap, sessionCounts, recallTotal, recallDue, lessonSet, now)
				category.nodes = append(category.nodes, node)
				flatCells = append(flatCells, node)
			}
			lang.categories = append(lang.categories, category)
		}

		if len(lang.categories) > 0 {
			languages = append(languages, lang)
		}
	}

	return languages, flatCells, nil
}

func buildGraphNode(
	topic models.Topic,
	parentTitle, langTitle string,
	progressMap map[string]models.TopicProgress,
	sessionCounts map[string]int,
	recallTotal, recallDue map[string]int,
	lessonSet map[string]bool,
	now time.Time,
) graphTopicNode {
	node := graphTopicNode{
		topicID:       topic.ID,
		title:         topic.Title,
		parentTitle:   parentTitle,
		languageTitle: langTitle,
	}

	if p, ok := progressMap[topic.ID]; ok {
		p = confidence.ApplyTimeDecay(p, now)
		node.hasProgress = true
		node.masteryLevel = p.MasteryLevel
		node.confidenceMod = p.ConfidenceModifier
		node.effectiveMastery = confidence.EffectiveMastery(p)
		node.aiExposureCount = p.AIExposureCount
		node.authoredCount = p.AuthoredCount
		node.lastConfirmedAt = p.LastConfirmedAt
	} else {
		node.confidenceMod = 1.0
	}

	node.sessionCount = sessionCounts[topic.ID]
	node.totalRecallCount = recallTotal[topic.ID]
	node.dueRecallCount = recallDue[topic.ID]
	node.inLessonQueue = lessonSet[topic.ID]

	return node
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

