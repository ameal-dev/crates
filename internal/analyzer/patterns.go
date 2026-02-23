package analyzer

import (
	"regexp"
	"sort"
)

// ConceptMatch represents a detected concept in a code diff.
type ConceptMatch struct {
	TopicSlug  string  `json:"slug"`
	Confidence float64 `json:"confidence"`
	CodeSnippet string `json:"snippet"`
}

type patternDef struct {
	re         *regexp.Regexp
	slug       string
	confidence float64
}

var patterns = []patternDef{
	// Go concurrency
	{regexp.MustCompile(`go\s+func|sync\.WaitGroup`), "go-goroutines", 0.85},
	{regexp.MustCompile(`\bchan\b|make\(chan|<-`), "go-channels", 0.80},
	{regexp.MustCompile(`\bselect\s*\{`), "go-select", 0.85},
	{regexp.MustCompile(`context\.(Background|WithCancel|WithTimeout|WithDeadline|WithValue|TODO)`), "go-context", 0.85},
	{regexp.MustCompile(`sync\.(Mutex|RWMutex|Once|Pool)\b|\.Lock\(\)|\.Unlock\(\)`), "go-sync", 0.85},

	// Go data structures
	{regexp.MustCompile(`\[\](\w+)|append\(|copy\(`), "go-arrays-slices", 0.75},
	{regexp.MustCompile(`map\[\w+\]\w+|make\(map\[`), "go-maps", 0.75},
	{regexp.MustCompile(`type\s+\w+\s+struct\s*\{`), "go-structs", 0.80},
	{regexp.MustCompile(`\*\w+\.|\&\w+`), "go-pointers", 0.70},

	// Go interfaces
	{regexp.MustCompile(`type\s+\w+\s+interface\s*\{`), "go-interface-basics", 0.85},
	{regexp.MustCompile(`io\.(Reader|Writer|ReadWriter|Closer)`), "go-io-interfaces", 0.80},

	// Go error handling
	{regexp.MustCompile(`fmt\.Errorf|errors\.(New|Is|As|Unwrap)`), "go-errors-basics", 0.80},
	{regexp.MustCompile(`errors\.(Is|As)\(`), "go-custom-errors", 0.80},

	// Go generics
	{regexp.MustCompile(`\[\w+\s+(any|comparable|~)`), "go-type-params", 0.85},

	// JavaScript async
	{regexp.MustCompile(`Promise\.(all|race|any|allSettled)|new\s+Promise`), "js-promises", 0.80},
	{regexp.MustCompile(`async\s+function|await\s+`), "js-async-await", 0.80},

	// React
	{regexp.MustCompile(`useEffect\s*\(`), "react-effects", 0.85},
	{regexp.MustCompile(`useState\s*[\(<]`), "react-state", 0.85},
	{regexp.MustCompile(`useContext\s*\(|createContext`), "react-context", 0.85},

	// TypeScript
	{regexp.MustCompile(`interface\s+\w+\s*\{`), "ts-interfaces", 0.75},
	{regexp.MustCompile(`type\s+\w+<\w+`), "ts-generics", 0.75},
}

// PatternMatch runs regex-based fast-path matching on a diff.
// Returns max 5 matches sorted by confidence descending, all >= 0.7.
func PatternMatch(diff string) []ConceptMatch {
	seen := make(map[string]ConceptMatch)

	for _, p := range patterns {
		loc := p.re.FindStringIndex(diff)
		if loc == nil {
			continue
		}
		if _, ok := seen[p.slug]; ok {
			continue
		}
		// Extract snippet: up to 100 chars around the match
		start := loc[0]
		end := loc[1]
		snippetStart := start - 20
		if snippetStart < 0 {
			snippetStart = 0
		}
		snippetEnd := end + 80
		if snippetEnd > len(diff) {
			snippetEnd = len(diff)
		}
		snippet := diff[snippetStart:snippetEnd]

		if p.confidence >= 0.7 {
			seen[p.slug] = ConceptMatch{
				TopicSlug:   p.slug,
				Confidence:  p.confidence,
				CodeSnippet: snippet,
			}
		}
	}

	matches := make([]ConceptMatch, 0, len(seen))
	for _, m := range seen {
		matches = append(matches, m)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Confidence > matches[j].Confidence
	})

	if len(matches) > 5 {
		matches = matches[:5]
	}

	return matches
}
