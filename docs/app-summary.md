# Kodsan — App Summary for Feature Iteration

## What It Is

Kodsan is a TUI-based Socratic coding tutor that runs alongside Claude Code. Claude Code writes code fast; Kodsan ensures the developer understands it. It detects concepts in code diffs, tracks understanding through interactive sessions, and uses spaced repetition to reinforce learning.

The binary runs in two mutually exclusive modes:
- **TUI mode** (default): Full-screen Bubble Tea terminal app
- **MCP mode** (`kodsan --mcp`): JSON-RPC server on stdio for Claude Code integration

## Tech Stack

- **Language:** Go 1.22+
- **TUI:** Bubble Tea v1, Lipgloss, Glamour, Bubbles (textinput, viewport, spinner)
- **AI:** Anthropic Go SDK — Claude Haiku 4.5 (primary), Sonnet 4.5 (fallback)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO), WAL mode
- **Migrations:** goose v3 with embedded SQL
- **Syntax highlighting:** Chroma v2 with Tokyo Night colors
- **MCP:** Official MCP Go SDK, stdio transport

## Screens (7 total)

### 1. Onboarding
Shown once on first launch. Three phases: pick language (Go/JS/TS/React), pick subcategory, API key check. Navigates directly into a session on the first leaf topic of the chosen subcategory. Can be skipped with `esc`.

### 2. Home (Dashboard)
The landing screen after onboarding. Three stat cards at top: "Recall Due", "Day Streak", "Mastered". Two-column layout below (when width >= 80): left column shows up to 3 lesson cards from the daily queue with hotkeys `[1]`/`[2]`/`[3]` to start, right column shows the 5 most recent sessions (navigable with `j/k`, resumable with `enter`). Bottom nav bar: `[t]` Topics, `[g]` Knowledge, `[r]` Recall, `[p]` Progress. Lesson queue is rebuilt every time home loads.

### 3. Session (Core Teaching)
The main learning interface. Header shows topic title + segmented mastery bar (5 blocks) + mastery badge. Scrollable chat area with markdown rendering (Glamour) and syntax-highlighted code blocks (Chroma). Single-line text input at bottom. Model indicator (Haiku/Sonnet) shown bottom-right.

**Teaching flow:**
1. Init loads topic, progress, recent code exposures; creates DB session
2. Sends "I want to learn about X. Start by asking me what I already know."
3. AI streams a Socratic response — questions, not lectures
4. On completion: parses `<!--CHECKPOINT:concept|evidence-->` markers from AI response, strips them, saves cleaned message, creates checkpoint records, increments mastery, updates confidence
5. User responds, AI continues the dialogue with full history
6. `ctrl+s` skips: records skip count, asks AI to explain directly instead

**Streaming:** Chunks arrive via a `chan tea.Msg`. Connecting phase shows animated spinner + "Connecting to Anthropic..." in both chat area and help bar. If Haiku is overloaded, client permanently switches to Sonnet for the rest of the session (no repeated retries).

**Session resume:** Navigating from home with a SessionID replays prior messages into chat view and AI context without re-streaming.

### 4. Topic Browser
Split view: collapsible tree on top, detail pane below. Four root topics (Go, JS, TS, React) expanded by default. `j/k` navigate, `l/right` expand, `h/left` collapse, `enter` on leaf starts session. `/` activates search: filters tree by case-insensitive title match. Mastery dots shown next to topics with progress. Only leaf topics can start sessions.

### 5. Recall (Spaced Repetition)
Loads up to 20 due cards ordered by `next_review`. Shows question in bordered card; `enter`/`space` reveals answer. Self-grade with `1`-`5` keys (forgot → easy). Runs SM-2 algorithm on grade, updates card scheduling. Grade >= 4 boosts confidence modifier by 0.1. Empty states: "No recall cards yet" vs "All caught up!"

### 6. Progress
Three hero stat cards: Mastered / In Progress / Not Started (counted from leaf topics). Below: hierarchical display with root topics as headers, category cards, leaf topics indented underneath. Each row shows: topic name + progress bar + "X/5" score + hint/skip counts. Bar color: green (clean), orange (hints/skips used), gray (zero mastery).

### 7. Knowledge Graph
Visual heatmap of all leaf topics as 2-character colored cells. Green `##` = effective mastery > 3.0, teal `::` = 2-3, yellow `//` = 1-2, red `..` = < 1.0, gray = untouched. Amber variant for high mastery but low confidence. Wide layout shows detail sidebar with confidence diagnostics. Summary banner at bottom. `j/k` navigate, `enter` starts session, `r` goes to recall.

## AI Integration

### Socratic Engine
`BuildSystemPrompt()` constructs a context-aware system prompt:
- Role: "You are a Socratic coding tutor. Teach through questions, not lectures."
- Current topic title + description
- Student progress: mastery level, hints used, skips
- If recent code exposures exist: includes actual code snippets from AI-accepted code and instructs the AI to use them as teaching examples
- 4-level hint ladder: nudge → pointed question → partial reveal → full explanation
- Checkpoint markers: AI emits `<!--CHECKPOINT:concept|evidence-->` when student demonstrates understanding
- Adapts approach by mastery: new (0), seen-but-unconfirmed (0 with exposures), beginner (1-2), intermediate (3-4), advanced (5)

### Checkpoint Parser
Regex extracts `<!--CHECKPOINT:concept|evidence-->` from AI responses. Returns cleaned text (markers removed) + parsed checkpoints. Markers are invisible to the user.

### Recall Card Generator
Takes checkpoints from a session, sends a single non-streaming call to generate Q&A flashcard pairs as JSON. Creates 1-2 cards per checkpoint. (Note: exists but not yet wired into the session completion flow.)

## Curriculum

40 leaf topics across 4 languages, 3-level hierarchy:

| Language | Categories | Leaf Topics |
|----------|-----------|-------------|
| Go | Basics, Data Structures, Interfaces, Concurrency, Error Handling, Generics, Testing | 22 |
| JavaScript | Basics, Async, Modules | 8 |
| TypeScript | Basics, Advanced | 5 |
| React | Basics, Patterns | 5 |

Topic IDs are human-readable slugs: `go-goroutines`, `react-state`, `typescript-mapped-types`, etc. Seeded on every startup via upsert (idempotent).

## Mastery & Confidence Model

### Core Formula
```
effective_mastery = mastery_level × confidence_modifier
```
This is the signal used for ALL filtering, prioritization, and display decisions.

### mastery_level
Integer 0–5. Incremented when AI emits a checkpoint marker during a session. Capped at 5.

### confidence_modifier
Float 0.3–1.0. Encodes *how* understanding was formed:

| Event | Effect |
|-------|--------|
| Session with confirmed checkpoint | Reset to 1.0, set `last_confirmed_at` |
| AI_ACCEPTED exposure (code accepted without review) | −0.15 per exposure (floor 0.3) |
| AUTHORED exposure (user wrote the code) | +0.05 (cap 1.0) |
| Recall card graded ≥ 4 | +0.1 |
| Time decay | −0.02 per week since `last_confirmed_at` |

**The key insight:** A topic with mastery_level=4 but confidence_modifier=0.3 has effective_mastery=1.2 — it shows as "shaky" despite high raw mastery. This happens when someone learned a concept months ago, then repeatedly accepted AI-generated code using that concept without review. The confidence modifier captures understanding erosion.

## Concept Extractor (Hybrid)

Maps code diffs to curriculum topic slugs. Two-phase pipeline:

**Fast path (regex patterns):** 21 patterns covering Go concurrency, data structures, interfaces, error handling, generics, JS async, React hooks, TypeScript types. Each maps to a topic slug with fixed confidence (0.70–0.85). Zero API cost, sub-millisecond.

**Slow path (Claude classification):** If fast path finds < 2 matches, sends diff to Haiku with a structured prompt listing all valid slugs. Returns `{slug, confidence, snippet}` array.

**Rules:**
- Discard matches with confidence < 0.7
- Max 5 matches per diff (top 5 by confidence)
- Cache by SHA-256 of diff content, 24-hour TTL
- Only match concepts that exist in the topics table
- Snippets truncated to 500 chars

## MCP Server (Claude Code Integration)

Two tools, both must respond within 2 seconds:

### `kodsan_get_user_context`
- **Input:** none
- **Output:** `{weak_topics: [{slug, title, effective_mastery}], mastery_summary: string}`
- Returns max 5 weak topics (effective_mastery < 2.0 after time decay)
- Called at start of Claude Code session to calibrate inline comments
- Claude Code's system prompt gets: "User has weak understanding of X. When generating code involving these concepts, add inline comments explaining the reasoning."

### `kodsan_record_exposure`
- **Input:** `{diff, commit_sha, source: "AUTHORED" | "AI_ACCEPTED"}`
- **Output:** `{status: "accepted", concepts_detected: [...], lessons_queued: N}`
- Returns immediately; concept extraction runs async in background goroutine
- For each detected concept: inserts `concept_exposure` record, updates `topic_progress` confidence modifier
- Fires after each accepted code generation or commit

## Lesson Queue

Rebuilt every time the home screen loads. Priority formula:

```
priority = gap_weight × recency_weight × frequency_weight
```

| Factor | Logic |
|--------|-------|
| gap_weight | effective=0: 1.0, <1.0: 0.9, 1-2: 0.6, 2-3: 0.3, >3: 0.1 |
| recency_weight | ≤ 1 day: 2.0×, linear decay to 1.0 over 7 days |
| frequency_weight | 1.0 + 0.1 × ai_exposure_count, capped at 1.5 |

**Hard constraints:**
- Topics with effective_mastery > 3.0 never enter queue (recall cards only)
- Maximum 3 items per day
- Queue cleared and rebuilt from scratch each time

## Database Schema (8 tables)

| Table | Purpose |
|-------|---------|
| `topics` | Curriculum tree (id, parent_id, slug, title, description, sort_order) |
| `sessions` | Learning sessions (id, topic_id, started_at, last_activity_at, status) |
| `messages` | Chat history (id, session_id, role, content, created_at) |
| `checkpoints` | Knowledge checkpoints (id, session_id, concept, evidence, mastery_delta) |
| `topic_progress` | Per-topic mastery + confidence state (mastery_level, confidence_modifier, ai_exposure_count, authored_count, hints_used, skips, last_confirmed_at) |
| `recall_cards` | Spaced repetition cards (id, topic_id, question, answer, next_review, interval_days, ease_factor, repetitions) |
| `concept_exposures` | Code-derived concept encounters (id, topic_id, source, commit_sha, file_path, code_snippet, confidence) |
| `lesson_queue` | Prioritized daily queue (topic_id, priority_score, source_exposure_id, queued_at) |
| `app_state` | Key-value store for persistent settings (onboarding_completed, etc.) |

SQLite config: WAL mode, foreign keys on, busy_timeout=5000ms, MaxOpenConns=1. Timestamps as ISO 8601 strings.

## UI Styling — Tokyo Night Palette

| Color | Hex | Usage |
|-------|-----|-------|
| Surface | `#1e2030` | Card/panel backgrounds |
| Accent (purple) | `#bb9af7` | Headers, key hints, accent bars, spinner |
| Pink | `#f7768e` | Errors, advanced badges, weak mastery |
| Orange | `#e0af68` | Warnings, intermediate badges, questions |
| Green | `#9ece6a` | Success, solid mastery, input prompt |
| Blue | `#7aa2f7` | User messages, info stats |
| Muted | `#565f89` | Descriptions, secondary text |
| Foreground | `#c0caf5` | Primary text |
| Dim border | `#3b3d57` | Card borders, separators |

Chat messages are styled with colored accent bars by role. Code blocks use Chroma syntax highlighting with topic-aware keyword highlighting (e.g., studying "go-channels" highlights `chan` in amber). Questions (lines ending with `?`) get an amber `>` prompt prefix.

## Known Gaps / Incomplete Areas

1. **Session completion:** Sessions are created as "active" but never transition to "completed" or "abandoned" — no explicit end-of-session flow, summary, or card generation trigger
2. **Recall card generation:** `GenerateRecallCards()` exists but is never called in the codebase — checkpoints are created during sessions but cards are never auto-generated from them
3. **Complexity weight:** The priority formula spec mentions `complexity_weight` (leaf=1.0, parent=0.5) but it's not implemented in `queue/builder.go`
4. **Chat scrolling:** The chat viewport exists but keyboard scrolling (pgup/pgdn) is not wired to the user in the session screen
5. **Palette duplication:** Each screen redeclares the Tokyo Night palette with screen-prefixed variable names (`hmAccent`, `ssAccent`, `pgAccent`, etc.) — could be consolidated
6. **Topic browser filtering:** `addChildrenToFlatList` ignores the `parentExpanded` parameter, potentially adding hidden children to the flat list
