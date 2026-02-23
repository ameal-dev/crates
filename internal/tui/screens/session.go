package screens

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ameal-dev/crates/internal/ai"
	"github.com/ameal-dev/crates/internal/confidence"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/ameal-dev/crates/internal/tui/components"
	"github.com/google/uuid"
)

// SessionScreen is the core teaching interaction screen.
type SessionScreen struct {
	db       *sql.DB
	aiClient ai.Streamer
	topicID  string
	apiKey   string

	// If set, resume this session instead of creating a new one
	resumeSessionID string

	// State
	session         models.Session
	topic           models.Topic
	progress        models.TopicProgress
	exposures       []models.ConceptExposure
	chatView        components.ChatView
	input           textinput.Model
	spinner         spinner.Model
	streaming       bool
	connecting      bool // true until first StreamChunkMsg arrives
	streamBuf       string
	chunkChan       chan tea.Msg
	completed       bool // session has ended (completed or abandoned)
	generatingCards bool // recall cards being generated
	err             error

	// Active model name (set from StreamConnectingMsg)
	activeModel string

	// Chat history for AI context
	chatHistory []ai.ChatMessage

	// Tracks messages sent for first-session hint
	messagesSent int

	width, height int
}

// Session screen aliases for shared palette
var (
	ssGreen     = clrGreen
	ssMuted     = clrMuted
	ssDimBorder = clrDimBorder
	ssFg        = clrFg
	ssAccent    = clrAccent
	ssOrange    = clrOrange
	ssBlue      = clrBlue
	ssPink      = clrPink
	ssSurface   = clrSurfaceHL
)

var (
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ssAccent).
			Padding(0, 1)

	headerBarStyle = lipgloss.NewStyle().
			Background(ssSurface).
			Foreground(ssFg).
			Padding(0, 2).
			Bold(true)

	helpBarStyle = lipgloss.NewStyle().
			Foreground(ssMuted).
			MarginTop(0)
)

// NewSessionScreen creates a new session screen for the given topic.
func NewSessionScreen(db *sql.DB, aiClient ai.Streamer, topicID, apiKey string) *SessionScreen {
	ti := textinput.New()
	ti.Placeholder = "Type your answer..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ssAccent)

	return &SessionScreen{
		db:       db,
		aiClient: aiClient,
		topicID:  topicID,
		apiKey:   apiKey,
		input:    ti,
		spinner:  sp,
		chatView: components.NewChatView(80, 20),
	}
}

// SetResumeSession configures this screen to resume an existing session
// instead of creating a new one.
func (s *SessionScreen) SetResumeSession(sessionID string) {
	s.resumeSessionID = sessionID
}

func (s *SessionScreen) Init() tea.Cmd {
	// Load topic, create session, send initial AI message
	return tea.Batch(
		textinput.Blink,
		s.initSession(),
	)
}

func (s *SessionScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			s.Cleanup()
			return s, tea.Quit
		case "ctrl+h":
			if s.completed {
				return s, func() tea.Msg {
					return NavigateMsg{Screen: "home"}
				}
			}
			return s, s.abandonAndGoHome()
		case "ctrl+d":
			if s.streaming || s.completed || s.session.ID == "" {
				return s, nil
			}
			return s, s.completeSession()
		case "ctrl+s":
			if s.streaming || s.completed {
				return s, nil
			}
			// Skip: record skip and ask AI to explain
			s.progress.Skips++
			_ = queries.UpsertTopicProgress(s.db, s.progress)
			s.chatHistory = append(s.chatHistory, ai.ChatMessage{
				Role:    "user",
				Content: "[SKIP - please explain this concept directly]",
			})
			s.chatView.AddMessage("system", "Skipped — asking for explanation...")
			return s, s.startStreaming()
		case "enter":
			if s.completed && !s.generatingCards {
				return s, func() tea.Msg {
					return NavigateMsg{Screen: "home"}
				}
			}
			if s.streaming || s.completed {
				return s, nil
			}
			input := strings.TrimSpace(s.input.Value())
			if input == "" {
				return s, nil
			}
			s.input.SetValue("")

			// Save user message
			msgID := uuid.New().String()
			_ = queries.InsertMessage(s.db, msgID, s.session.ID, "user", input)
			_ = queries.UpdateSessionActivity(s.db, s.session.ID)

			s.chatHistory = append(s.chatHistory, ai.ChatMessage{
				Role:    "user",
				Content: input,
			})
			s.chatView.AddMessage("user", input)
			s.messagesSent++

			return s, s.startStreaming()
		case "pgup", "pgdown":
			s.chatView.Update(msg)
			return s, nil
		}

	case tea.MouseMsg:
		s.chatView.Update(msg)
		return s, nil

	case sessionInitMsg:
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.session = msg.session
		s.topic = msg.topic
		s.progress = msg.progress
		s.exposures = msg.exposures
		s.chatView.SetTopicSlug(s.topic.Slug)

		if msg.resumed && len(msg.messages) > 0 {
			// Replay prior conversation into chat view and AI context
			s.chatView.AddMessage("system", fmt.Sprintf("Resumed: %s — %s", s.topic.Title, s.topic.Description))
			for _, m := range msg.messages {
				if m.Role == "user" || m.Role == "assistant" {
					s.chatHistory = append(s.chatHistory, ai.ChatMessage{
						Role:    m.Role,
						Content: m.Content,
					})
					s.chatView.AddMessage(m.Role, m.Content)
					if m.Role == "user" {
						s.messagesSent++
					}
				}
			}
			// Don't auto-stream — user picks up where they left off
			return s, nil
		}

		// New session — show topic and start AI
		s.chatView.AddMessage("system", fmt.Sprintf("Topic: %s — %s", s.topic.Title, s.topic.Description))
		return s, s.startStreaming()

	case ai.StreamConnectingMsg:
		s.connecting = true
		s.activeModel = msg.Model
		s.chatView.SetStreamingContent("Connecting to Anthropic...")
		return s, tea.Batch(s.spinner.Tick, ai.WaitForChunk(s.chunkChan))

	case ai.StreamChunkMsg:
		s.connecting = false
		s.streamBuf += msg.Content
		s.chatView.SetStreamingContent(s.streamBuf)
		return s, ai.WaitForChunk(s.chunkChan)

	case ai.StreamDoneMsg:
		s.streaming = false
		fullContent := s.streamBuf
		s.streamBuf = ""
		s.chatView.ClearStreaming()

		if fullContent != "" {
			// Parse checkpoints from the response
			cleaned, checkpoints := ai.ParseCheckpoints(fullContent)

			// Save assistant message (cleaned)
			msgID := uuid.New().String()
			_ = queries.InsertMessage(s.db, msgID, s.session.ID, "assistant", cleaned)

			s.chatHistory = append(s.chatHistory, ai.ChatMessage{
				Role:    "assistant",
				Content: cleaned,
			})
			s.chatView.AddMessage("assistant", cleaned)

			// Process checkpoints
			for _, cp := range checkpoints {
				cpID := uuid.New().String()
				_ = queries.InsertCheckpoint(s.db, cpID, s.session.ID, cp.Concept, cp.Evidence, 1)
				_ = queries.IncrementMastery(s.db, s.topicID, 1)
			}

			// Refresh progress and update confidence after session checkpoint
			s.progress, _ = queries.GetTopicProgress(s.db, s.topicID)
			if len(checkpoints) > 0 {
				s.progress = confidence.UpdateAfterSession(s.progress)
				_ = queries.UpsertTopicProgress(s.db, s.progress)
			}

			// Auto-complete when mastery reaches max
			if s.progress.MasteryLevel >= 5 && !s.completed {
				return s, s.completeSession()
			}
		}
		return s, nil

	case sessionEndMsg:
		s.generatingCards = false
		if msg.err != nil {
			s.chatView.AddMessage("system", fmt.Sprintf("Could not generate recall cards: %v", msg.err))
		} else if msg.cardsGenerated > 0 {
			s.chatView.AddMessage("system", fmt.Sprintf("Generated %d recall cards for review.", msg.cardsGenerated))
		}
		s.chatView.AddMessage("system", "Press enter or ctrl+h to return home.")
		return s, nil

	case ai.StreamFallbackMsg:
		s.connecting = true
		s.chatView.AddMessage("system",
			fmt.Sprintf("Model %s unavailable, retrying with %s...", msg.FromModel, msg.ToModel))
		return s, ai.WaitForChunk(s.chunkChan)

	case ai.StreamErrorMsg:
		s.streaming = false
		s.connecting = false
		s.streamBuf = ""
		s.chatView.ClearStreaming()
		s.chatView.AddMessage("system", fmt.Sprintf("Error: %v", msg.Err))
		return s, nil

	case spinner.TickMsg:
		if s.connecting || s.generatingCards {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil
	}

	// Update text input
	if !s.streaming {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	return s, nil
}

func (s *SessionScreen) View() string {
	if s.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress ctrl+h to go home.", s.err)
	}

	// Header bar
	header := s.renderHeader()

	// Chat area: header(1) + sep(1) + inputSep(1) + input(3, border top+content+bottom) + help(1)
	chatHeight := s.height - 7
	if chatHeight < 5 {
		chatHeight = 5
	}
	s.chatView.SetSize(s.width, chatHeight)

	// Input bar
	promptStyle := lipgloss.NewStyle().Foreground(ssGreen).Bold(true)
	s.input.Prompt = promptStyle.Render("> ")
	s.input.Width = s.width - 6
	inputSep := lipgloss.NewStyle().Foreground(ssDimBorder).Render(strings.Repeat("─", s.width))
	inputView := inputStyle.Width(s.width - 2).Render(s.input.View())

	// Help bar
	var helpLeft string
	if s.completed && s.generatingCards {
		helpLeft = s.spinner.View() + " Generating recall cards..."
	} else if s.completed {
		helpLeft = "enter home • ctrl+h home"
	} else if s.streaming && s.connecting {
		helpLeft = s.spinner.View() + " Connecting to Anthropic..."
	} else if s.streaming {
		helpLeft = "Streaming..."
	} else if s.messagesSent < 2 {
		helpLeft = "Tip: type your answer and press enter — it's okay to be wrong, that's how you learn."
	} else {
		helpLeft = "enter send • ctrl+s skip • ctrl+d done • ctrl+h home"
	}

	// Model indicator (right-aligned)
	var help string
	if s.activeModel != "" && s.width > 0 {
		modelTag := shortModelName(s.activeModel)
		modelStyle := lipgloss.NewStyle().Foreground(ssMuted)
		rightPart := modelStyle.Render(modelTag)
		leftRendered := helpBarStyle.Render(helpLeft)
		gap := s.width - lipgloss.Width(leftRendered) - lipgloss.Width(rightPart)
		if gap < 1 {
			gap = 1
		}
		help = leftRendered + strings.Repeat(" ", gap) + rightPart
	} else {
		help = helpBarStyle.Render(helpLeft)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		s.chatView.View(),
		inputSep,
		inputView,
		help,
	)
}

func (s *SessionScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *SessionScreen) renderHeader() string {
	if s.topic.Title == "" {
		return headerBarStyle.Width(s.width).Render("Loading...")
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ssFg)

	// Segmented progress bar: █░░░░
	filledStyle := lipgloss.NewStyle().Foreground(ssGreen)
	emptyStyle := lipgloss.NewStyle().Foreground(ssDimBorder)
	bar := filledStyle.Render(strings.Repeat("█", s.progress.MasteryLevel)) +
		emptyStyle.Render(strings.Repeat("░", 5-s.progress.MasteryLevel))

	badge := masteryBadge(s.progress.MasteryLevel)

	header := headerBarStyle.Width(s.width).Render(
		fmt.Sprintf("  %s  %s  %s",
			titleStyle.Render(s.topic.Title),
			bar,
			badge,
		),
	)

	sep := lipgloss.NewStyle().Foreground(ssDimBorder).Width(s.width).Render(
		strings.Repeat("─", s.width),
	)

	return header + "\n" + sep
}

func masteryBadge(level int) string {
	var label string
	var bg lipgloss.Color
	switch {
	case level == 0:
		label, bg = "unknown", ssMuted
	case level <= 1:
		label, bg = "weak", ssPink
	case level <= 2:
		label, bg = "shaky", ssOrange
	case level <= 3:
		label, bg = "good", ssBlue
	default:
		label, bg = "solid", ssGreen
	}
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(clrDark).
		Padding(0, 1).
		Render(label)
}

func shortModelName(model string) string {
	switch {
	case strings.Contains(model, "haiku"):
		return "Haiku"
	case strings.Contains(model, "sonnet"):
		return "Sonnet"
	case strings.Contains(model, "opus"):
		return "Opus"
	default:
		return model
	}
}

func (s *SessionScreen) startStreaming() tea.Cmd {
	if s.aiClient == nil {
		s.chatView.AddMessage("system", "No API key set. Set ANTHROPIC_API_KEY and restart.")
		return nil
	}

	s.streaming = true
	s.streamBuf = ""
	s.chunkChan = make(chan tea.Msg, 64)

	systemPrompt := ai.BuildSystemPrompt(s.topic, s.progress, s.exposures)

	// If no user messages yet, add an initial prompt
	history := s.chatHistory
	if len(history) == 0 {
		history = []ai.ChatMessage{{
			Role:    "user",
			Content: fmt.Sprintf("I want to learn about %s. Start by asking me what I already know about this topic.", s.topic.Title),
		}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	return tea.Batch(
		func() tea.Msg {
			defer cancel()
			cmd := s.aiClient.StreamResponseWithChunks(
				ctx,
				systemPrompt,
				history,
				s.chunkChan,
			)
			return cmd()
		},
		ai.WaitForChunk(s.chunkChan),
	)
}

// sessionEndMsg reports the result of recall card generation.
type sessionEndMsg struct {
	cardsGenerated int
	err            error
}

// Cleanup marks the session as abandoned if still active. Called on app exit.
func (s *SessionScreen) Cleanup() {
	if s.session.ID != "" && !s.completed {
		_ = queries.UpdateSessionStatus(s.db, s.session.ID, "abandoned")
	}
}

// abandonAndGoHome marks the session abandoned and navigates home.
// Recall card generation runs as fire-and-forget (saves to DB in background).
func (s *SessionScreen) abandonAndGoHome() tea.Cmd {
	if s.session.ID != "" {
		_ = queries.UpdateSessionStatus(s.db, s.session.ID, "abandoned")
	}
	s.completed = true

	navCmd := func() tea.Msg {
		return NavigateMsg{Screen: "home"}
	}

	recallCmd := s.fireRecallGeneration()
	if recallCmd != nil {
		return tea.Batch(recallCmd, navCmd)
	}
	return navCmd
}

// completeSession marks the session completed, shows a summary, and generates recall cards.
func (s *SessionScreen) completeSession() tea.Cmd {
	if s.session.ID != "" {
		_ = queries.UpdateSessionStatus(s.db, s.session.ID, "completed")
	}
	s.completed = true

	checkpoints, _ := queries.GetCheckpointsForSession(s.db, s.session.ID)
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Session complete — %d checkpoints, mastery: %d/5", len(checkpoints), s.progress.MasteryLevel))
	if len(checkpoints) > 0 {
		summary.WriteString("\n\nConcepts covered:")
		for _, cp := range checkpoints {
			summary.WriteString(fmt.Sprintf("\n  - %s", cp.Concept))
		}
	}
	s.chatView.AddMessage("system", summary.String())

	cmd := s.fireRecallGeneration()
	if cmd != nil {
		s.generatingCards = true
		s.chatView.AddMessage("system", "Generating recall cards...")
		return tea.Batch(cmd, s.spinner.Tick)
	}

	s.chatView.AddMessage("system", "Press enter or ctrl+h to return home.")
	return nil
}

// fireRecallGeneration returns a tea.Cmd that generates recall cards from session checkpoints.
func (s *SessionScreen) fireRecallGeneration() tea.Cmd {
	if s.apiKey == "" || s.session.ID == "" {
		return nil
	}

	checkpoints, _ := queries.GetCheckpointsForSession(s.db, s.session.ID)
	if len(checkpoints) == 0 {
		return nil
	}

	topicTitle := s.topic.Title
	topicID := s.topicID
	db := s.db
	apiKey := s.apiKey

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cards, err := ai.GenerateRecallCards(ctx, apiKey, topicTitle, checkpoints)
		if err != nil {
			return sessionEndMsg{err: err}
		}

		for _, card := range cards {
			id := uuid.New().String()
			_ = queries.InsertRecallCard(db, id, topicID, card.Question, card.Answer)
		}

		return sessionEndMsg{cardsGenerated: len(cards)}
	}
}

// initSession loads topic data and creates a DB session.
type sessionInitMsg struct {
	session   models.Session
	topic     models.Topic
	progress  models.TopicProgress
	exposures []models.ConceptExposure
	messages  []models.Message // populated on resume
	resumed   bool
	err       error
}

func (s *SessionScreen) initSession() tea.Cmd {
	return func() tea.Msg {
		// Resume path: reopen an existing session
		if s.resumeSessionID != "" {
			session, err := queries.GetSession(s.db, s.resumeSessionID)
			if err != nil {
				return sessionInitMsg{err: fmt.Errorf("load session: %w", err)}
			}

			// Reactivate if completed/abandoned
			if session.Status != "active" {
				_ = queries.UpdateSessionStatus(s.db, session.ID, "active")
				session.Status = "active"
			}
			_ = queries.UpdateSessionActivity(s.db, session.ID)

			topic, err := queries.GetTopic(s.db, session.TopicID)
			if err != nil {
				return sessionInitMsg{err: fmt.Errorf("load topic: %w", err)}
			}

			progress, err := queries.GetTopicProgress(s.db, session.TopicID)
			if err != nil {
				return sessionInitMsg{err: fmt.Errorf("load progress: %w", err)}
			}

			msgs, err := queries.GetMessagesForSession(s.db, session.ID)
			if err != nil {
				return sessionInitMsg{err: fmt.Errorf("load messages: %w", err)}
			}

			exposures, _ := queries.GetExposuresForTopic(s.db, session.TopicID, 5)

			return sessionInitMsg{
				session:   session,
				topic:     topic,
				progress:  progress,
				exposures: exposures,
				messages:  msgs,
				resumed:   true,
			}
		}

		// New session path
		topic, err := queries.GetTopic(s.db, s.topicID)
		if err != nil {
			return sessionInitMsg{err: fmt.Errorf("load topic: %w", err)}
		}

		progress, err := queries.GetTopicProgress(s.db, s.topicID)
		if err != nil {
			return sessionInitMsg{err: fmt.Errorf("load progress: %w", err)}
		}

		sessionID := uuid.New().String()
		session, err := queries.CreateSession(s.db, sessionID, s.topicID)
		if err != nil {
			return sessionInitMsg{err: fmt.Errorf("create session: %w", err)}
		}

		// Load recent exposures for code context (non-fatal if absent)
		exposures, _ := queries.GetExposuresForTopic(s.db, s.topicID, 5)

		return sessionInitMsg{session: session, topic: topic, progress: progress, exposures: exposures}
	}
}
