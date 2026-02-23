package ai

import (
	"regexp"
	"strings"
)

// ParsedCheckpoint represents a knowledge checkpoint extracted from AI output.
type ParsedCheckpoint struct {
	Concept  string
	Evidence string
}

var checkpointRegex = regexp.MustCompile(`<!--CHECKPOINT:([^|]+)\|([^>]*)-->`)

// ParseCheckpoints extracts checkpoint markers from text and returns the
// cleaned text (markers removed) along with parsed checkpoints.
func ParseCheckpoints(text string) (string, []ParsedCheckpoint) {
	matches := checkpointRegex.FindAllStringSubmatch(text, -1)
	var checkpoints []ParsedCheckpoint
	for _, m := range matches {
		checkpoints = append(checkpoints, ParsedCheckpoint{
			Concept:  strings.TrimSpace(m[1]),
			Evidence: strings.TrimSpace(m[2]),
		})
	}

	cleaned := checkpointRegex.ReplaceAllString(text, "")
	// Collapse runs of 3+ newlines down to 2 (one blank line)
	multiNewline := regexp.MustCompile(`\n{3,}`)
	cleaned = multiNewline.ReplaceAllString(cleaned, "\n\n")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned, checkpoints
}
