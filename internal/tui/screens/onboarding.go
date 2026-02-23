package screens

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
)

const (
	onboardingPhaseLang    = 0
	onboardingPhaseSub     = 1
	onboardingPhaseAPIKey  = 2
)

// OnboardingScreen guides first-time users to start a session in 2 steps.
type OnboardingScreen struct {
	db        *sql.DB
	hasAPIKey bool

	phase         int
	languages     []models.Topic // root-level topics (Go, JavaScript, etc.)
	langIdx       int
	subcategories []models.Topic // children of selected language
	subIdx        int

	width, height int
	err           error
}

type onboardingDataMsg struct {
	languages []models.Topic
	err       error
}

// NewOnboardingScreen creates a new onboarding screen.
func NewOnboardingScreen(db *sql.DB, hasAPIKey bool) *OnboardingScreen {
	return &OnboardingScreen{
		db:        db,
		hasAPIKey: hasAPIKey,
	}
}

func (o *OnboardingScreen) Init() tea.Cmd {
	return o.loadLanguages()
}

func (o *OnboardingScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.SetSize(msg.Width, msg.Height)
		return o, nil

	case tea.KeyMsg:
		switch o.phase {
		case onboardingPhaseLang:
			return o.updateLangPhase(msg)
		case onboardingPhaseSub:
			return o.updateSubPhase(msg)
		case onboardingPhaseAPIKey:
			return o.updateAPIKeyPhase(msg)
		}

	case onboardingDataMsg:
		if msg.err != nil {
			o.err = msg.err
			return o, nil
		}
		o.languages = msg.languages
		return o, nil
	}

	return o, nil
}

func (o *OnboardingScreen) updateLangPhase(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if o.langIdx < len(o.languages)-1 {
			o.langIdx++
		}
	case "k", "up":
		if o.langIdx > 0 {
			o.langIdx--
		}
	case "enter":
		if len(o.languages) > 0 {
			selected := o.languages[o.langIdx]
			o.subcategories = selected.Children
			o.subIdx = 0
			o.phase = onboardingPhaseSub
		}
	case "esc":
		// Skip onboarding, mark complete, go home
		_ = queries.SetAppState(o.db, "onboarding_completed", "true")
		return o, func() tea.Msg {
			return NavigateMsg{Screen: "home"}
		}
	case "q", "ctrl+c":
		return o, tea.Quit
	}
	return o, nil
}

func (o *OnboardingScreen) updateSubPhase(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if o.subIdx < len(o.subcategories)-1 {
			o.subIdx++
		}
	case "k", "up":
		if o.subIdx > 0 {
			o.subIdx--
		}
	case "enter":
		if len(o.subcategories) == 0 {
			return o, nil
		}
		if !o.hasAPIKey {
			o.phase = onboardingPhaseAPIKey
			return o, nil
		}
		selected := o.subcategories[o.subIdx]
		topicID := firstLeafTopic(selected)
		_ = queries.SetAppState(o.db, "onboarding_completed", "true")
		return o, func() tea.Msg {
			return NavigateMsg{Screen: "session", TopicID: topicID}
		}
	case "esc":
		o.phase = onboardingPhaseLang
		o.subIdx = 0
	case "q", "ctrl+c":
		return o, tea.Quit
	}
	return o, nil
}

func (o *OnboardingScreen) updateAPIKeyPhase(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return o, tea.Quit
	}
	return o, nil
}

func (o *OnboardingScreen) View() string {
	if o.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress ctrl+c to quit.", o.err)
	}

	switch o.phase {
	case onboardingPhaseLang:
		return o.viewLangPhase()
	case onboardingPhaseSub:
		return o.viewSubPhase()
	case onboardingPhaseAPIKey:
		return o.viewAPIKeyPhase()
	}
	return ""
}

func (o *OnboardingScreen) viewLangPhase() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  crates"))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("  Learn programming through Socratic dialogue."))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  You answer questions. The AI guides you to understanding."))
	b.WriteString("\n\n")
	b.WriteString("  What do you want to learn first?\n\n")

	for i, lang := range o.languages {
		cursor := "    "
		name := lang.Title
		if i == o.langIdx {
			cursor = "  > "
			name = selectedStyle.Render(lang.Title)
		}
		desc := descStyle.Render(lang.Description)
		b.WriteString(fmt.Sprintf("%s%-16s%s\n", cursor, name, desc))
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  ↑↓ choose • enter confirm • esc skip"))
	b.WriteString("\n")

	return b.String()
}

func (o *OnboardingScreen) viewSubPhase() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	langTitle := ""
	if o.langIdx < len(o.languages) {
		langTitle = o.languages[o.langIdx].Title
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("  %s — pick a starting topic", langTitle)))
	b.WriteString("\n\n")

	for i, sub := range o.subcategories {
		cursor := "    "
		name := sub.Title
		if i == o.subIdx {
			cursor = "  > "
			name = selectedStyle.Render(sub.Title)
		}
		desc := descStyle.Render(sub.Description)
		b.WriteString(fmt.Sprintf("%s%-20s%s\n", cursor, name, desc))
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  ↑↓ choose • enter confirm • esc back"))
	b.WriteString("\n")

	return b.String()
}

func (o *OnboardingScreen) viewAPIKeyPhase() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B"))

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  crates needs an Anthropic API key to work."))
	b.WriteString("\n\n")
	b.WriteString("  Set it in your shell and relaunch:\n\n")
	b.WriteString("    " + codeStyle.Render(`export ANTHROPIC_API_KEY="sk-ant-..."`) + "\n\n")
	b.WriteString(subtleStyle.Render("  Get a key at console.anthropic.com"))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("  q quit"))
	b.WriteString("\n")

	return b.String()
}

func (o *OnboardingScreen) SetSize(width, height int) {
	o.width = width
	o.height = height
}

func (o *OnboardingScreen) loadLanguages() tea.Cmd {
	return func() tea.Msg {
		tree, err := queries.GetTopicTree(o.db)
		if err != nil {
			return onboardingDataMsg{err: err}
		}
		return onboardingDataMsg{languages: tree}
	}
}

// firstLeafTopic walks down the first child of each level until it finds a leaf.
func firstLeafTopic(topic models.Topic) string {
	if len(topic.Children) == 0 {
		return topic.ID
	}
	return firstLeafTopic(topic.Children[0])
}

// HasAPIKey checks if the ANTHROPIC_API_KEY environment variable is set.
func HasAPIKey() bool {
	return os.Getenv("ANTHROPIC_API_KEY") != ""
}
