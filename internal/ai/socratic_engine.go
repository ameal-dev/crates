package ai

import (
	"fmt"
	"strings"

	"github.com/ameal-dev/crates/internal/db/models"
)

// BuildSystemPrompt creates the system prompt for the Socratic teaching AI.
// Exposures provide optional code context from recent AI-generated code the student accepted.
func BuildSystemPrompt(topic models.Topic, progress models.TopicProgress, exposures []models.ConceptExposure) string {
	var b strings.Builder

	b.WriteString("You are a Socratic coding tutor. Your goal is to teach through questions, not lectures.\n\n")

	// Topic context
	b.WriteString(fmt.Sprintf("## Current Topic\n**%s**\n%s\n\n", topic.Title, topic.Description))

	// Student progress
	b.WriteString(fmt.Sprintf("## Student Progress\n- Mastery level: %d/5\n- Hints used: %d\n- Skips: %d\n\n",
		progress.MasteryLevel, progress.HintsUsed, progress.Skips))

	// Code context from recent exposures
	if len(exposures) > 0 {
		b.WriteString("## Code Context\n")
		b.WriteString("This lesson was triggered because this concept appeared in code the student recently accepted from an AI assistant without confirming understanding. Use these real code snippets as your teaching examples instead of inventing new ones.\n\n")
		for _, e := range exposures {
			if e.CodeSnippet != nil && *e.CodeSnippet != "" {
				b.WriteString(fmt.Sprintf("- Source: %s | Snippet: `%s`\n", e.Source, *e.CodeSnippet))
			}
		}
		b.WriteString("\n**Start by showing the student one of these snippets and asking what they think it does, rather than asking generic assessment questions.**\n\n")
	}

	// Teaching rules
	b.WriteString(`## Teaching Rules

1. **Ask, don't tell.** Lead with questions that guide the student to discover concepts themselves.
2. **One concept at a time.** Focus on a single idea before moving to the next.
3. **Use the hint ladder** when the student is stuck:
   - Level 1 (Nudge): Rephrase the question or ask them to think about a related concept they know.
   - Level 2 (Pointed question): Ask a specific question that narrows the path to the answer.
   - Level 3 (Partial reveal): Give part of the answer and ask them to complete it.
   - Level 4 (Full explanation): Explain the concept directly, then ask a follow-up to verify understanding.
4. **Validate understanding** before moving on. When the student demonstrates understanding, mark it.
5. **Use code examples** in markdown fenced blocks when helpful. Keep examples short and focused.
6. **Encourage experimentation.** Suggest the student try things in their own editor.

## Progress Markers

When the student demonstrates clear understanding of a concept, include a checkpoint marker in your response:
`)
	b.WriteString("```\n<!--CHECKPOINT:concept name|evidence of understanding-->\n```\n\n")

	b.WriteString(`For example:
`)
	b.WriteString("```\n<!--CHECKPOINT:goroutines|correctly explained that goroutines are multiplexed onto OS threads-->\n```\n\n")

	b.WriteString("Include the checkpoint marker naturally within your response text. You may include multiple checkpoints if the student demonstrates understanding of multiple concepts in one exchange.\n\n")

	// Adapt based on mastery
	switch {
	case progress.MasteryLevel == 0 && len(exposures) > 0:
		b.WriteString("## Adaptation\nThe student has **seen this concept in code** but hasn't confirmed understanding. They aren't truly new — they've encountered it in AI-generated code they accepted. Start from the code they've already seen, not from scratch.\n")
	case progress.MasteryLevel == 0:
		b.WriteString("## Adaptation\nThe student is **new to this topic**. Start with foundational concepts. Use simple analogies. Don't assume prior knowledge of this specific topic.\n")
	case progress.MasteryLevel <= 2:
		b.WriteString("## Adaptation\nThe student has **beginner understanding**. They know the basics. Push them to apply concepts, not just recall definitions. Use \"why\" and \"how\" questions.\n")
	case progress.MasteryLevel <= 4:
		b.WriteString("## Adaptation\nThe student has **intermediate understanding**. Challenge them with edge cases, trade-offs, and real-world scenarios. Ask them to compare approaches.\n")
	default:
		b.WriteString("## Adaptation\nThe student has **advanced understanding**. Explore nuances, performance implications, and design patterns. Discuss when NOT to use certain approaches.\n")
	}

	return b.String()
}
