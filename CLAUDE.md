# Kodsan — Socratic Coding Tutor + Understanding Layer

Kodsan is the understanding layer that runs alongside AI-assisted coding. Claude Code writes
the code; Kodsan ensures the developer understands it. These two tools are designed to be
used together — Claude Code accelerates output, Kodsan ensures understanding accumulates
alongside that output rather than being replaced by it.

## Tech Stack

- **Language:** Go 1.22+
- **TUI:** Bubble Tea v1, Lipgloss, Glamour, Bubbles (textinput, viewport)
- **AI:** Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Migrations:** goose/v3 with embedded SQL files
- **Testing:** `go test -race`

## Project Structure

```
cmd/kodsan/main.go        → Entry point: open DB, migrate, seed, start TUI
internal/
  db/
    db.go                 → Open(), RunMigrations() — WAL, FK, busy_timeout
    models/models.go      → 8 structs: Topic, Session, Message, Checkpoint,
                            TopicProgress, RecallCard, ConceptExposure, LessonQueueItem
    queries/              → Pure query functions: topics.go, sessions.go, messages.go,
                            checkpoints.go, progress.go, recall.go, exposures.go,
                            lesson_queue.go
  ai/
    client.go             → Anthropic SDK wrapper, StreamResponse() → tea.Cmd
    messages.go           → StreamChunkMsg, StreamDoneMsg, StreamErrorMsg
    socratic_engine.go    → BuildSystemPrompt() with hint ladder + progress markers
    checkpoint_parser.go  → Extract <!--CHECKPOINT:...--> markers
    recall_generator.go   → Generate Q&A cards from checkpoints
  analyzer/
    extractor.go          → Maps code diffs to topic slugs via Claude classification
    patterns.go           → Regex-based fast-path matchers for common concepts
    slugs.go              → LeafSlugs() extracts leaf topic slugs from curriculum tree
    cache.go              → Diff hash → classification cache (avoid redundant API calls)
  confidence/
    model.go              → EffectiveMastery, UpdateAfterExposure/Session/Recall, ApplyTimeDecay
  mcp/
    server.go             → MCP server: kodsan_get_user_context + kodsan_record_exposure
    handlers.go           → Tool handler implementations
  queue/
    builder.go            → BuildDailyQueue() — priority scoring, max 3 lessons/day
  tui/
    app.go                → Root model, screen switching, WindowSizeMsg propagation
    screens/
      home.go             → Dashboard: recent sessions, recall count, mastery overview,
                            today's lesson queue
      session.go          → Streaming chat with progress header + textinput
      topic_browser.go    → Split-pane: tree left, detail right
      recall.go           → Card queue, reveal, self-grade
      progress.go         → Per-topic mastery, hint/skip counts, recall stats
    components/
      chat_view.go        → Viewport + Glamour rendering
      topic_tree.go       → Collapsible tree with search + mastery badges
      progress_bar.go     → Segmented mastery bar with color gradient
  curriculum/seed.go      → SeedCurriculum: walk topic tree, upsert to DB
  spaced/sm2.go           → SM-2 algorithm
data/curriculum.go        → Topic tree as Go struct literals
migrations/
  001_initial_schema.sql  → Original 6 tables with indexes
  002_app_state.sql       → app_state key-value table
  003_exposures_and_queue.sql → concept_exposures + lesson_queue tables, topic_progress extensions
```

## Commands

```bash
make build      # go build -o bin/kodsan ./cmd/kodsan
make run        # go run ./cmd/kodsan
make test       # go test ./...
make test-race  # go test -race ./...
make lint       # golangci-lint run
make clean      # rm -rf bin/
make mcp        # go run ./cmd/kodsan --mcp  (start MCP server mode)
```

---

## Architecture Rules

- **Elm architecture:** Every screen is a Bubble Tea `Model` with `Init()`, `Update()`, `View()`.
- **Screen switching:** Root model holds current screen enum. Navigation via messages
  (`NavigateMsg{Screen}`), not direct model swaps.
- **Streaming:** AI responses stream via `tea.Cmd` that sends `StreamChunkMsg` per delta,
  `StreamDoneMsg` on completion. Never use goroutines directly in TUI code.
- **DB access:** Plain query functions `func Foo(db *sql.DB, ...) (T, error)`. No ORM.
- **Section rendering:** Each `View()` returns a string. Compose with Lipgloss
  `JoinVertical`/`JoinHorizontal`.

---

## NEVER Do

- **NEVER** use goroutines in TUI code — use `tea.Cmd` for async work
- **NEVER** log to stdout/stderr — Bubble Tea owns the terminal. Use a file logger if needed
- **NEVER** call `os.Exit()` in models — return `tea.Quit` command instead
- **NEVER** open multiple SQLite connections — use `SetMaxOpenConns(1)`
- **NEVER** store raw AI responses without stripping `<!--CHECKPOINT:...-->` markers first
- **ALL** screens MUST handle `tea.WindowSizeMsg` to support terminal resizing
- **NEVER** treat `AI_ACCEPTED` and `SESSION` exposures as equivalent mastery signals —
  they are fundamentally different and must be stored and weighted separately
- **NEVER** queue more than 3 lessons per day — the tool must feel effortless, not like
  homework. Ruthless filtering is a feature, not a limitation
- **NEVER** surface a concept with effective_mastery > 3.0 as a lesson (recall cards only)
- **NEVER** call the concept extractor synchronously in the MCP `record_exposure` handler —
  always async. The handler must return within 2 seconds
- **NEVER** match trivial code patterns in the extractor (variable declarations, bare
  imports) — only match when a concept is non-trivially present, meaning actively used
  with meaningful logic, not just referenced
- **NEVER** queue more than 5 concept matches per diff — filter by confidence and pick
  the most significant to avoid noise overwhelming the lesson queue

---

## DB Schema (8 tables)

| Table               | Purpose                                                                                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------- |
| `topics`            | Curriculum tree (id, parent_id, slug, title, description, sort_order)                                     |
| `sessions`          | Learning sessions (id, topic_id, started_at, last_activity_at, status)                                    |
| `messages`          | Chat history (id, session_id, role, content, created_at)                                                  |
| `checkpoints`       | Knowledge checkpoints (id, session_id, concept, evidence, mastery_delta)                                  |
| `topic_progress`    | Per-topic mastery state — see extended columns below                                                      |
| `recall_cards`      | Spaced repetition cards (id, topic_id, question, answer, next_review, interval, ease_factor, repetitions) |
| `concept_exposures` | Code-derived concept encounters — see schema below                                                        |
| `lesson_queue`      | Prioritized daily lesson queue (topic_id, priority_score, source_exposure_id, queued_at)                  |

### topic_progress Extended Columns (migration 002)

```sql
-- Original columns retained unchanged
-- New columns added in 003_exposures_and_queue.sql:
ai_exposure_count    INTEGER NOT NULL DEFAULT 0,
authored_count       INTEGER NOT NULL DEFAULT 0,
last_exposure_source TEXT DEFAULT 'SESSION',  -- SESSION | AUTHORED | AI_ACCEPTED
confidence_modifier  REAL NOT NULL DEFAULT 1.0,
last_confirmed_at    DATETIME
```

### concept_exposures Schema

```sql
CREATE TABLE concept_exposures (
  id            TEXT PRIMARY KEY,
  topic_id      TEXT NOT NULL REFERENCES topics(id),
  source        TEXT NOT NULL,   -- SESSION | AUTHORED | AI_ACCEPTED
  commit_sha    TEXT,
  file_path     TEXT,
  code_snippet  TEXT,            -- relevant snippet, max 500 chars
  confidence    REAL NOT NULL,   -- 0.0–1.0 from classifier
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_exposures_topic ON concept_exposures(topic_id);
CREATE INDEX idx_exposures_created ON concept_exposures(created_at);
```

---

## Mastery Confidence Model

This is the core logic that determines what gets surfaced as a lesson. All filtering
decisions flow through effective mastery — never raw mastery_level alone.

```
effective_mastery = mastery_level × confidence_modifier
```

### Confidence Modifier Rules

| Event                             | Effect on confidence_modifier            |
| --------------------------------- | ---------------------------------------- |
| Session with confirmed checkpoint | Set to 1.0                               |
| AI_ACCEPTED exposure (no session) | -= 0.15 per exposure, floor 0.3          |
| AUTHORED exposure                 | += 0.05, cap 1.0                         |
| Recall card graded ≥ 4            | += 0.1                                   |
| Time decay                        | -= 0.02 per week since last_confirmed_at |

The rationale: a concept you mastered in a session 8 months ago that has since appeared
10 times in AI-generated code you accepted without review should not be treated as
understood. The confidence modifier encodes _how_ understanding was formed, not just
whether it was ever assessed.

### Lesson Queue Priority Formula

```
priority_score =
    gap_weight          -- UNKNOWN=1.0, effective<1.0=0.9, effective 1-2=0.6, effective>3=0.1
  × recency_weight      -- appeared in last 24h = 2.0x multiplier, decays daily
  × frequency_weight    -- appeared 5+ times in code this week = 1.5x
  × complexity_weight   -- leaf topic = 1.0, parent/category topic = 0.5
```

**Daily queue cap: 3 concepts maximum.**
Concepts with effective_mastery > 3.0 never enter the lesson queue — recall cards only.

---

## Concept Extractor

`internal/analyzer/extractor.go` maps code diffs to topic slugs.

### Approach: Hybrid

**Fast path (patterns.go):** AST-based matchers for the 20 most common curriculum
concepts. Zero API cost, sub-millisecond. Run this first.

**Slow path (extractor.go):** Send unmatched diff to Claude with structured prompt.
Returns `[]ConceptMatch{TopicSlug, Confidence, CodeSnippet}`. Run async, never blocking.

### Claude Classification Prompt Contract

The prompt must instruct Claude to return only a JSON array — no prose, no markdown
fences. Example response shape:

```json
[
  {
    "slug": "react/hooks/useeffect",
    "confidence": 0.92,
    "snippet": "useEffect(() => {...}, [userId])"
  },
  {
    "slug": "react/hooks/stale-closure",
    "confidence": 0.85,
    "snippet": "captures userId without dep"
  }
]
```

### Extractor Rules

- Discard any match with confidence < 0.7
- Max 5 matches per diff — take top 5 by confidence if more are found
- Cache classifications by SHA-256 of the diff content to avoid redundant API calls
- Cache TTL: 24 hours
- Only match concepts that exist in the topics table — discard unknown slugs
- Snippets are truncated to 500 chars max before storage

### Topic Slug Format

`language/category/concept` — always lowercase, hyphens for spaces.

Examples:

- `react/hooks/useeffect`
- `go/concurrency/goroutines`
- `typescript/generics/constraints`
- `javascript/async/event-loop`

---

## MCP Server

`internal/mcp/server.go` exposes Kodsan's knowledge graph to Claude Code. This is the
integration point that makes Kodsan and Claude Code aware of each other.

### Transport

Standard MCP stdio transport. Start with: `kodsan --mcp`

Claude Code MCP config entry:

```json
{
  "mcpServers": {
    "kodsan": {
      "command": "/usr/local/bin/kodsan",
      "args": ["--mcp"]
    }
  }
}
```

### Two Tools Only

Keep the MCP surface area minimal. Complexity here breaks the 2-second response budget.

**`kodsan_get_user_context`**

- Input: none
- Output: `{weak_topics: TopicSummary[], mastery_summary: string}`
- Returns max 5 weak topics (effective_mastery < 2.0), ordered by priority_score
- Used by Claude Code in its system prompt to calibrate generation style and comments
- Called at the start of a Claude Code session, not on every generation

**`kodsan_record_exposure`**

- Input: `{diff: string, commit_sha: string, source: "AUTHORED" | "AI_ACCEPTED"}`
- Output: `{concepts_detected: string[], lessons_queued: int}`
- Returns immediately (async extraction in background goroutine)
- Fires after each accepted code generation or commit

### MCP Rules

- Both tools MUST respond within 2 seconds — no exceptions
- `record_exposure` returns immediately; concept extraction runs in background
- The SQLite connection is shared with the main TUI process via WAL mode —
  this is safe with `busy_timeout=5000` already configured
- Never start the TUI and MCP server simultaneously in the same process —
  they are separate run modes (`kodsan` vs `kodsan --mcp`)

### What Claude Code Does With Context

When `kodsan_get_user_context` returns weak topics, Claude Code's system prompt gains:

```
User has weak understanding of: react/hooks/useeffect (effective mastery: 0.8),
go/concurrency/channels (effective mastery: 1.1)

When generating code involving these concepts, add inline comments explaining
the reasoning, not just the mechanics.
```

This makes Claude Code a slightly better teacher for this specific user's gaps without
any manual configuration.

---

## Bubble Tea Patterns

### Streaming from AI

```go
func streamCmd(client *ai.Client, msgs []Message) tea.Cmd {
    return func() tea.Msg {
        // This runs in a goroutine managed by Bubble Tea
        ch := client.StreamResponse(ctx, msgs)
        // Return first chunk; subsequent chunks via tea.Batch or channel polling
    }
}
```

### Screen Navigation

```go
type NavigateMsg struct{ Screen ScreenType }

// In root Update:
case NavigateMsg:
    m.currentScreen = msg.Screen
    return m, m.screens[msg.Screen].Init()
```

### Keybinds

- `ctrl+c` — quit (handled in root model)
- `ctrl+h` — navigate home
- `ctrl+s` — skip current question (session screen)
- `enter` — submit input / select item
- `tab` — cycle focus
- `↑↓` — navigate lists/trees
- `←→` — collapse/expand tree nodes

---

## SQLite Conventions

- **WAL mode:** `PRAGMA journal_mode=WAL` on every connection open
- **Foreign keys:** `PRAGMA foreign_keys=ON`
- **Busy timeout:** `PRAGMA busy_timeout=5000`
- **Timestamps:** ISO 8601 strings (`2006-01-02T15:04:05Z07:00`)
- **Migrations:** Embedded via `//go:embed`, run with goose on startup
- **Driver name:** `"sqlite"` (modernc.org/sqlite), NOT `"sqlite3"`
- **Max connections:** `SetMaxOpenConns(1)` — always, including when MCP server is running
  alongside TUI (WAL handles the concurrency safely)

---

## Streaming API Patterns

- Create a single `*anthropic.Client` at startup, pass it through
- Use `client.Messages.New()` for single responses,
  `client.Messages.NewStreaming()` for streaming
- Bridge to Bubble Tea: streaming goroutine sends chunks over a channel,
  a `tea.Cmd` reads from the channel
- Handle empty deltas (content block start/stop events with no text)
- For MCP handlers: use `client.Messages.New()` (non-streaming) — no TUI involved

---

## Gotchas

- **SQLite driver name** is `"sqlite"` not `"sqlite3"` when using modernc.org/sqlite
- **Use `textinput`** from Bubbles, not `textarea` — single-line input is the right UX
- **Glamour adds trailing newlines** — trim them before composing with Lipgloss
- **Use `lipgloss.Width()`** for measuring rendered string widths (handles ANSI)
- **Goose annotations:** Migration files need `-- +goose Up` and `-- +goose Down` comments
- **Empty streaming deltas:** The Anthropic SDK may emit content block events with empty
  text — guard against appending empty strings
- **Long topic names:** Truncate with ellipsis in tree view when width is constrained
- **MCP stdio:** The MCP server uses stdin/stdout for the JSON-RPC transport — any debug
  logging MUST go to a file, never stdout/stderr, or it corrupts the protocol
- **Confidence modifier floor:** Never let confidence_modifier drop below 0.3 — a floor
  ensures heavily AI-exposed topics still appear in sessions rather than disappearing
- **Diff size limit:** Truncate diffs sent to the classifier at 4000 tokens — larger diffs
  produce noisy, low-confidence classifications and cost more than they're worth
- **Background goroutines in MCP:** The `record_exposure` handler spawns a goroutine for
  async extraction — use a `sync.WaitGroup` or context with timeout to ensure clean
  shutdown when the MCP server exits

---

## Terminology

- **Topic** — A node in the curriculum tree (e.g., "Go / Concurrency / Goroutines")
- **Session** — A single teaching conversation about a topic
- **Checkpoint** — A verified understanding point extracted from AI dialogue
- **Recall card** — A Q&A flashcard generated from checkpoints for spaced repetition
- **Mastery level** — Integer 0–5 tracking understanding depth per topic
- **Effective mastery** — `mastery_level × confidence_modifier` — the real signal used
  for all filtering and prioritization decisions
- **Confidence modifier** — A 0.3–1.0 multiplier reflecting _how_ understanding was
  formed, not just whether it was ever assessed
- **Hint ladder** — 4 escalating hint levels: nudge → pointed question →
  partial reveal → full explanation
- **Progress marker** — `<!--CHECKPOINT:concept|evidence-->` embedded in AI responses
- **Concept exposure** — An instance of a topic appearing in code, regardless of whether
  it was understood
- **AI_ACCEPTED** — A concept encountered in Claude Code output that was accepted without
  a Kodsan session confirming understanding. Treated as weaker signal than SESSION
- **AUTHORED** — A concept the user wrote themselves, without AI generation
- **SESSION** — A concept confirmed through a Kodsan Socratic session with checkpoint
- **Lesson queue** — The prioritized list of concepts due for a Socratic session,
  derived from code exposures filtered through the mastery confidence model

---

## Build Order

**Phases 1–14 (complete):**
DB layer, curriculum, AI client, Socratic engine, TUI screens, checkpoint processing,
recall card generation, SM-2, spaced repetition screen, schema extension
(`003_exposures_and_queue.sql`), concept extractor (hybrid regex + Claude classification),
MCP server (`kodsan --mcp`), mastery confidence model, lesson queue + home screen integration.

---

## Skill Reference

See `.agents/skills/golang-pro/references/` for Go patterns:

- `concurrency.md` — goroutines, channels, select
- `interfaces.md` — interface design, composition
- `generics.md` — type parameters, constraints
- `testing.md` — table-driven tests, benchmarks
- `project-structure.md` — module layout, internal packages
