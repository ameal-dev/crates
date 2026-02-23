package analyzer

import "github.com/ameal-dev/crates/internal/db/models"

// LeafSlugs collects slugs from leaf topics (those with no children).
func LeafSlugs(topics []models.Topic) []string {
	var slugs []string
	collectLeaves(topics, &slugs)
	return slugs
}

func collectLeaves(topics []models.Topic, slugs *[]string) {
	for _, t := range topics {
		if len(t.Children) == 0 {
			*slugs = append(*slugs, t.Slug)
		} else {
			collectLeaves(t.Children, slugs)
		}
	}
}
