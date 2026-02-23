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

// TopicBrowserScreen shows the curriculum tree with a detail pane.
type TopicBrowserScreen struct {
	db            *sql.DB
	topics        []models.Topic
	flatList      []flatTopic
	selectedIdx   int
	progressMap   map[string]models.TopicProgress
	searchQuery   string
	searching     bool
	width, height int
	err           error
}

type flatTopic struct {
	topic models.Topic
	depth int
	expanded bool
	hasChildren bool
}

type topicDataMsg struct {
	topics   []models.Topic
	progress []models.TopicProgress
	err      error
}

// NewTopicBrowserScreen creates a new topic browser.
func NewTopicBrowserScreen(db *sql.DB) *TopicBrowserScreen {
	return &TopicBrowserScreen{
		db:          db,
		progressMap: make(map[string]models.TopicProgress),
	}
}

func (t *TopicBrowserScreen) Init() tea.Cmd {
	return t.loadTopics()
}

func (t *TopicBrowserScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if t.selectedIdx < len(t.flatList)-1 {
				t.selectedIdx++
			}
		case "k", "up":
			if t.selectedIdx > 0 {
				t.selectedIdx--
			}
		case "l", "right":
			if t.selectedIdx < len(t.flatList) && t.flatList[t.selectedIdx].hasChildren {
				t.flatList[t.selectedIdx].expanded = true
				t.rebuildFlatList()
			}
		case "h", "left":
			if t.selectedIdx < len(t.flatList) {
				if t.flatList[t.selectedIdx].expanded {
					t.flatList[t.selectedIdx].expanded = false
					t.rebuildFlatList()
				}
			}
		case "enter":
			if t.selectedIdx < len(t.flatList) {
				topic := t.flatList[t.selectedIdx].topic
				// Only start sessions on leaf topics
				if !t.flatList[t.selectedIdx].hasChildren {
					return t, func() tea.Msg {
						return NavigateMsg{Screen: "session", TopicID: topic.ID}
					}
				}
				// Toggle expand for non-leaf
				t.flatList[t.selectedIdx].expanded = !t.flatList[t.selectedIdx].expanded
				t.rebuildFlatList()
			}
		case "/":
			t.searching = true
		case "esc":
			if t.searching {
				t.searching = false
				t.searchQuery = ""
				t.rebuildFlatList()
			}
		case "backspace":
			if t.searching && len(t.searchQuery) > 0 {
				t.searchQuery = t.searchQuery[:len(t.searchQuery)-1]
				t.rebuildFlatList()
			}
		default:
			if t.searching && len(msg.String()) == 1 {
				t.searchQuery += msg.String()
				t.rebuildFlatList()
			}
		}

	case topicDataMsg:
		if msg.err != nil {
			t.err = msg.err
			return t, nil
		}
		t.topics = msg.topics
		for _, p := range msg.progress {
			t.progressMap[p.TopicID] = p
		}
		t.buildInitialFlatList()
		return t, nil
	}

	return t, nil
}

func (t *TopicBrowserScreen) View() string {
	if t.err != nil {
		return fmt.Sprintf("Error: %v", t.err)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		MarginBottom(1)

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Background(lipgloss.Color("#374151"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B5CF6")).
		Bold(true)

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Topics"))
	b.WriteString("\n")

	if t.searching {
		b.WriteString(fmt.Sprintf("  🔍 %s▊\n\n", t.searchQuery))
	}

	// Tree view (left side)
	treeWidth := t.width / 2
	if treeWidth < 30 {
		treeWidth = t.width
	}

	for i, ft := range t.flatList {
		indent := strings.Repeat("  ", ft.depth+1)
		prefix := "  "
		if ft.hasChildren {
			if ft.expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		}

		mastery := ""
		if p, ok := t.progressMap[ft.topic.ID]; ok && p.MasteryLevel > 0 {
			mastery = fmt.Sprintf(" %s", renderMasteryBadge(p.MasteryLevel))
		}

		line := fmt.Sprintf("%s%s%s%s", indent, prefix, ft.topic.Title, mastery)

		// Truncate if too wide
		if lipgloss.Width(line) > treeWidth-2 {
			line = line[:treeWidth-5] + "..."
		}

		if i == t.selectedIdx {
			line = selectedStyle.Width(treeWidth).Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Detail pane
	if t.selectedIdx < len(t.flatList) {
		selected := t.flatList[t.selectedIdx].topic
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("  Description: "))
		b.WriteString(selected.Description)
		b.WriteString("\n")
		if p, ok := t.progressMap[selected.ID]; ok {
			b.WriteString(fmt.Sprintf("  Mastery: %d/5  Hints: %d  Skips: %d\n",
				p.MasteryLevel, p.HintsUsed, p.Skips))
		}
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("  %s navigate • %s expand/collapse • %s start session • %s search • %s home",
		keyStyle.Render("↑↓"),
		keyStyle.Render("←→"),
		keyStyle.Render("enter"),
		keyStyle.Render("/"),
		keyStyle.Render("ctrl+h"),
	)))

	return b.String()
}

func (t *TopicBrowserScreen) SetSize(width, height int) {
	t.width = width
	t.height = height
}

func (t *TopicBrowserScreen) loadTopics() tea.Cmd {
	return func() tea.Msg {
		topics, err := queries.GetTopicTree(t.db)
		if err != nil {
			return topicDataMsg{err: err}
		}
		progress, err := queries.GetAllTopicProgress(t.db)
		if err != nil {
			return topicDataMsg{err: err}
		}
		return topicDataMsg{topics: topics, progress: progress}
	}
}

func (t *TopicBrowserScreen) buildInitialFlatList() {
	t.flatList = nil
	for _, topic := range t.topics {
		t.flatList = append(t.flatList, flatTopic{
			topic:       topic,
			depth:       0,
			expanded:    true, // top-level expanded by default
			hasChildren: len(topic.Children) > 0,
		})
		if len(topic.Children) > 0 {
			t.addChildrenToFlatList(topic.Children, 1, false) // second level collapsed
		}
	}
}

func (t *TopicBrowserScreen) addChildrenToFlatList(children []models.Topic, depth int, parentExpanded bool) {
	for _, child := range children {
		ft := flatTopic{
			topic:       child,
			depth:       depth,
			expanded:    false,
			hasChildren: len(child.Children) > 0,
		}
		t.flatList = append(t.flatList, ft)
	}
}

func (t *TopicBrowserScreen) rebuildFlatList() {
	// Save expansion state
	expandState := make(map[string]bool)
	for _, ft := range t.flatList {
		expandState[ft.topic.ID] = ft.expanded
	}

	t.flatList = nil
	searchLower := strings.ToLower(t.searchQuery)

	for _, topic := range t.topics {
		if t.searching && !t.topicMatchesSearch(topic, searchLower) {
			continue
		}
		expanded := expandState[topic.ID]
		t.flatList = append(t.flatList, flatTopic{
			topic:       topic,
			depth:       0,
			expanded:    expanded,
			hasChildren: len(topic.Children) > 0,
		})
		if expanded && len(topic.Children) > 0 {
			t.addChildrenRecursive(topic.Children, 1, expandState, searchLower)
		}
	}

	if t.selectedIdx >= len(t.flatList) {
		t.selectedIdx = max(0, len(t.flatList)-1)
	}
}

func (t *TopicBrowserScreen) addChildrenRecursive(children []models.Topic, depth int, expandState map[string]bool, searchLower string) {
	for _, child := range children {
		if t.searching && !t.topicMatchesSearch(child, searchLower) {
			continue
		}
		expanded := expandState[child.ID]
		t.flatList = append(t.flatList, flatTopic{
			topic:       child,
			depth:       depth,
			expanded:    expanded,
			hasChildren: len(child.Children) > 0,
		})
		if expanded && len(child.Children) > 0 {
			t.addChildrenRecursive(child.Children, depth+1, expandState, searchLower)
		}
	}
}

func (t *TopicBrowserScreen) topicMatchesSearch(topic models.Topic, searchLower string) bool {
	if strings.Contains(strings.ToLower(topic.Title), searchLower) {
		return true
	}
	for _, child := range topic.Children {
		if t.topicMatchesSearch(child, searchLower) {
			return true
		}
	}
	return false
}

func renderMasteryBadge(level int) string {
	colors := []string{"#6B7280", "#EAB308", "#F59E0B", "#22C55E", "#10B981", "#06B6D4"}
	if level < 0 || level >= len(colors) {
		level = 0
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[level]))
	dots := strings.Repeat("●", level) + strings.Repeat("○", 5-level)
	return style.Render(dots)
}
