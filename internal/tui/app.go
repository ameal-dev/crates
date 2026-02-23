package tui

import (
	"database/sql"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/internal/tui/screens"
)

// ScreenType identifies which screen is currently displayed.
type ScreenType int

const (
	ScreenHome ScreenType = iota
	ScreenTopicBrowser
	ScreenSession
	ScreenRecall
	ScreenProgress
	ScreenOnboarding
	ScreenKnowledgeGraph
)

var screenNameMap = map[string]ScreenType{
	"home":          ScreenHome,
	"topic_browser": ScreenTopicBrowser,
	"session":       ScreenSession,
	"recall":        ScreenRecall,
	"progress":      ScreenProgress,
	"onboarding":        ScreenOnboarding,
	"knowledge_graph":   ScreenKnowledgeGraph,
}

// App is the root Bubble Tea model.
type App struct {
	db            *sql.DB
	aiClient      ai.Streamer
	apiKey        string
	currentScreen ScreenType
	screenModels  map[ScreenType]screens.Screen
	width, height int
	ready         bool
}

// NewApp creates the root application model.
func NewApp(db *sql.DB, aiClient ai.Streamer, apiKey string) App {
	return App{
		db:            db,
		aiClient:      aiClient,
		apiKey:        apiKey,
		currentScreen: ScreenHome,
		screenModels:  make(map[ScreenType]screens.Screen),
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		// Initialize screens on first resize
		if len(a.screenModels) == 0 {
			a.initScreens()
		}

		// Propagate to all screens
		for _, s := range a.screenModels {
			s.SetSize(a.width, a.height)
		}

		// Init current screen on first load
		if s, ok := a.screenModels[a.currentScreen]; ok {
			return a, s.Init()
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if cleaner, ok := a.screenModels[a.currentScreen].(screens.Cleanable); ok {
				cleaner.Cleanup()
			}
			return a, tea.Quit
		case "ctrl+h":
			// Let session screen handle its own cleanup flow
			if a.currentScreen == ScreenSession {
				break
			}
			if a.currentScreen != ScreenHome && a.currentScreen != ScreenOnboarding {
				a.currentScreen = ScreenHome
				if s, ok := a.screenModels[ScreenHome]; ok {
					return a, s.Init()
				}
			}
			return a, nil
		}

	case screens.NavigateMsg:
		st, ok := screenNameMap[msg.Screen]
		if !ok {
			return a, nil
		}
		a.currentScreen = st

		// If navigating to session with a topic, create a new session screen
		if st == ScreenSession && msg.TopicID != "" {
			sess := screens.NewSessionScreen(a.db, a.aiClient, msg.TopicID, a.apiKey)
			if msg.SessionID != "" {
				sess.SetResumeSession(msg.SessionID)
			}
			sess.SetSize(a.width, a.height)
			a.screenModels[ScreenSession] = sess
			return a, sess.Init()
		}

		if screen, ok := a.screenModels[st]; ok {
			return a, screen.Init()
		}
		return a, nil
	}

	// Delegate to current screen
	if !a.ready {
		return a, nil
	}
	if screen, ok := a.screenModels[a.currentScreen]; ok {
		updated, cmd := screen.Update(msg)
		a.screenModels[a.currentScreen] = updated
		return a, cmd
	}

	return a, nil
}

const (
	minWidth  = 60
	minHeight = 15
)

func (a App) View() string {
	if !a.ready {
		return "Loading..."
	}

	if a.width < minWidth || a.height < minHeight {
		msg := fmt.Sprintf(
			"Terminal too small (%d×%d). Minimum: %d×%d.\n\nResize your terminal to continue.",
			a.width, a.height, minWidth, minHeight,
		)
		style := lipgloss.NewStyle().
			Width(a.width).
			Height(a.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(lipgloss.Color("#e0af68"))
		return style.Render(msg)
	}

	if screen, ok := a.screenModels[a.currentScreen]; ok {
		return screen.View()
	}

	return "No screen loaded"
}

func (a *App) initScreens() {
	home := screens.NewHomeScreen(a.db)
	home.SetSize(a.width, a.height)
	a.screenModels[ScreenHome] = home

	browser := screens.NewTopicBrowserScreen(a.db)
	browser.SetSize(a.width, a.height)
	a.screenModels[ScreenTopicBrowser] = browser

	progress := screens.NewProgressScreen(a.db)
	progress.SetSize(a.width, a.height)
	a.screenModels[ScreenProgress] = progress

	recall := screens.NewRecallScreen(a.db, a.aiClient)
	recall.SetSize(a.width, a.height)
	a.screenModels[ScreenRecall] = recall

	knowledgeGraph := screens.NewKnowledgeGraphScreen(a.db)
	knowledgeGraph.SetSize(a.width, a.height)
	a.screenModels[ScreenKnowledgeGraph] = knowledgeGraph

	// Check if this is a first launch: no sessions and onboarding not completed
	completed, _ := queries.GetAppState(a.db, "onboarding_completed")
	if completed != "true" {
		sessions, _ := queries.GetRecentSessions(a.db, 1)
		if len(sessions) == 0 {
			onboarding := screens.NewOnboardingScreen(a.db, a.aiClient != nil)
			onboarding.SetSize(a.width, a.height)
			a.screenModels[ScreenOnboarding] = onboarding
			a.currentScreen = ScreenOnboarding
		}
	}
}

// Styles used across screens
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	AccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	KeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B5CF6")).
			Bold(true)
)
