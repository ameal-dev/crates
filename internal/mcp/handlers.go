package mcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ameal-dev/crates/internal/analyzer"
	"github.com/ameal-dev/crates/internal/confidence"
	"github.com/ameal-dev/crates/internal/db/models"
	"github.com/ameal-dev/crates/internal/db/queries"
	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool input/output types

type getUserContextInput struct{}

type topicSummary struct {
	Slug             string  `json:"slug"`
	Title            string  `json:"title"`
	EffectiveMastery float64 `json:"effective_mastery"`
}

type getUserContextOutput struct {
	WeakTopics []topicSummary `json:"weak_topics"`
}

type recordExposureInput struct {
	Diff      string `json:"diff" jsonschema:"code diff to analyze for concepts"`
	CommitSHA string `json:"commit_sha" jsonschema:"git commit SHA"`
	Source    string `json:"source" jsonschema:"how the code was created: AUTHORED or AI_ACCEPTED"`
}

type recordExposureOutput struct {
	Status string `json:"status"`
}

func (s *Server) registerTools() {
	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "crates_get_user_context",
		Description: "Returns the user's weak topics and mastery summary for calibrating code generation style",
	}, s.handleGetUserContext)

	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "crates_record_exposure",
		Description: "Records a concept exposure from code diff (AUTHORED or AI_ACCEPTED source)",
	}, s.handleRecordExposure)
}

func (s *Server) handleGetUserContext(ctx context.Context, req *mcpsdk.CallToolRequest, _ getUserContextInput) (*mcpsdk.CallToolResult, getUserContextOutput, error) {
	allProgress, err := queries.GetAllTopicProgress(s.db)
	if err != nil {
		return nil, getUserContextOutput{}, fmt.Errorf("get progress: %w", err)
	}

	now := time.Now()

	// Build topic title map
	topicTree, err := queries.GetTopicTree(s.db)
	if err != nil {
		return nil, getUserContextOutput{}, fmt.Errorf("get topics: %w", err)
	}
	titleMap := buildTitleMap(topicTree)

	type ranked struct {
		slug    string
		title   string
		effMast float64
	}

	var weak []ranked
	for _, p := range allProgress {
		p = confidence.ApplyTimeDecay(p, now)
		eff := confidence.EffectiveMastery(p)
		if eff < 2.0 {
			title := p.TopicID
			if t, ok := titleMap[p.TopicID]; ok {
				title = t
			}
			weak = append(weak, ranked{slug: p.TopicID, title: title, effMast: eff})
		}
	}

	sort.Slice(weak, func(i, j int) bool {
		return weak[i].effMast < weak[j].effMast
	})

	limit := 5
	if len(weak) < limit {
		limit = len(weak)
	}

	result := getUserContextOutput{
		WeakTopics: make([]topicSummary, limit),
	}
	for i := 0; i < limit; i++ {
		result.WeakTopics[i] = topicSummary{
			Slug:             weak[i].slug,
			Title:            weak[i].title,
			EffectiveMastery: weak[i].effMast,
		}
	}

	return nil, result, nil
}

func (s *Server) handleRecordExposure(ctx context.Context, req *mcpsdk.CallToolRequest, input recordExposureInput) (*mcpsdk.CallToolResult, recordExposureOutput, error) {
	if input.Source != "AUTHORED" && input.Source != "AI_ACCEPTED" {
		return nil, recordExposureOutput{}, fmt.Errorf("invalid source: %q (must be AUTHORED or AI_ACCEPTED)", input.Source)
	}

	// Fire background goroutine for async extraction
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processExposure(input)
	}()

	return nil, recordExposureOutput{Status: "accepted"}, nil
}

func (s *Server) processExposure(input recordExposureInput) {
	topicTree, err := queries.GetTopicTree(s.db)
	if err != nil {
		return
	}
	validSlugs := analyzer.LeafSlugs(topicTree)

	matches, err := analyzer.AnalyzeDiff(context.Background(), s.apiKey, input.Diff, validSlugs, nil)
	if err != nil {
		return
	}

	now := time.Now().UTC()
	for _, m := range matches {
		// Insert exposure
		var commitSHA *string
		if input.CommitSHA != "" {
			commitSHA = &input.CommitSHA
		}
		snippet := m.CodeSnippet
		exposure := models.ConceptExposure{
			ID:          uuid.New().String(),
			TopicID:     m.TopicSlug,
			Source:      input.Source,
			CommitSHA:   commitSHA,
			CodeSnippet: &snippet,
			Confidence:  m.Confidence,
			CreatedAt:   now,
		}
		if err := queries.InsertExposure(s.db, exposure); err != nil {
			continue
		}

		// Update confidence
		prog, err := queries.GetTopicProgress(s.db, m.TopicSlug)
		if err != nil {
			continue
		}
		prog = confidence.UpdateAfterExposure(prog, input.Source)
		_ = queries.UpsertTopicProgress(s.db, prog)
	}
}

func buildTitleMap(topics []models.Topic) map[string]string {
	m := make(map[string]string)
	var walk func([]models.Topic)
	walk = func(ts []models.Topic) {
		for _, t := range ts {
			m[t.ID] = t.Title
			walk(t.Children)
		}
	}
	walk(topics)
	return m
}
