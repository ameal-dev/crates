package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night code block palette
var (
	cbBg       = lipgloss.Color("#24283b")
	cbAccent   = lipgloss.Color("#bb9af7")
	cbMuted    = lipgloss.Color("#565f89")
	cbBorder   = lipgloss.Color("#3b3d57")
	cbFg       = lipgloss.Color("#a9b1d6")
	cbTermBg   = lipgloss.Color("#e0af68")
	cbTermFg   = lipgloss.Color("#1a1b26")
	cbHeaderFg = lipgloss.Color("#565f89")
)

// Token type → lipgloss color mapping (Tokyo Night)
var tokenColors = map[chroma.TokenType]lipgloss.Color{
	chroma.Keyword:          lipgloss.Color("#bb9af7"),
	chroma.KeywordConstant:  lipgloss.Color("#bb9af7"),
	chroma.KeywordType:      lipgloss.Color("#2ac3de"),
	chroma.KeywordNamespace: lipgloss.Color("#bb9af7"),
	chroma.NameBuiltin:      lipgloss.Color("#2ac3de"),
	chroma.NameFunction:     lipgloss.Color("#7aa2f7"),
	chroma.NameClass:        lipgloss.Color("#2ac3de"),
	chroma.NameDecorator:    lipgloss.Color("#9ece6a"),
	chroma.NameOther:        lipgloss.Color("#a9b1d6"),
	chroma.LiteralString:    lipgloss.Color("#9ece6a"),
	chroma.LiteralNumber:    lipgloss.Color("#ff9e64"),
	chroma.Comment:          lipgloss.Color("#565f89"),
	chroma.CommentSingle:    lipgloss.Color("#565f89"),
	chroma.Operator:         lipgloss.Color("#89ddff"),
	chroma.Punctuation:      lipgloss.Color("#a9b1d6"),
}

var fenceRe = regexp.MustCompile("^```(\\w*)\\s*$")

// Segment represents either a prose or code portion of markdown content.
type Segment struct {
	IsCode   bool
	Language string
	Content  string
}

// ExtractSegments splits markdown content into alternating prose and code segments.
// If the buffer ends with an unclosed fence, the trailing content stays as prose
// (streaming safety).
func ExtractSegments(markdown string) []Segment {
	lines := strings.Split(markdown, "\n")
	var segments []Segment
	var current strings.Builder
	inCode := false
	lang := ""

	for _, line := range lines {
		if !inCode {
			if m := fenceRe.FindStringSubmatch(line); m != nil {
				// Flush prose
				if current.Len() > 0 {
					segments = append(segments, Segment{Content: current.String()})
					current.Reset()
				}
				inCode = true
				lang = m[1]
				continue
			}
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(line)
		} else {
			if strings.TrimSpace(line) == "```" {
				// Close code block
				segments = append(segments, Segment{
					IsCode:   true,
					Language: lang,
					Content:  current.String(),
				})
				current.Reset()
				inCode = false
				lang = ""
				continue
			}
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(line)
		}
	}

	// Flush remaining content
	if current.Len() > 0 {
		if inCode {
			// Unclosed fence — treat as prose (streaming safety)
			segments = append(segments, Segment{Content: "```" + lang + "\n" + current.String()})
		} else {
			segments = append(segments, Segment{Content: current.String()})
		}
	}

	return segments
}

// RenderCodeBlock produces a styled code block with line numbers, syntax highlighting,
// accent bar, header, and rounded border.
func RenderCodeBlock(code, language string, width int, highlightTerms []string) string {
	if width < 20 {
		width = 20
	}

	// Tokenize with Chroma
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		// Fallback: plain text
		return renderPlainCodeBlock(code, language, width)
	}

	// Render tokens with lipgloss styles
	termHighlight := lipgloss.NewStyle().Background(cbTermBg).Foreground(cbTermFg)
	defaultStyle := lipgloss.NewStyle().Foreground(cbFg)

	var highlighted strings.Builder
	for _, token := range iterator.Tokens() {
		text := token.Value
		style := defaultStyle
		if c, ok := tokenColors[token.Type]; ok {
			style = lipgloss.NewStyle().Foreground(c)
		} else if c, ok := tokenColors[token.Type.Category()]; ok {
			style = lipgloss.NewStyle().Foreground(c)
		}
		if token.Type == chroma.Comment || token.Type == chroma.CommentSingle {
			style = style.Italic(true)
		}

		// Apply term highlighting
		if len(highlightTerms) > 0 {
			highlighted.WriteString(highlightInText(text, highlightTerms, style, termHighlight))
		} else {
			highlighted.WriteString(style.Render(text))
		}
	}

	return buildCodeFrame(highlighted.String(), language, width)
}

// highlightInText applies term highlighting within a token's text.
func highlightInText(text string, terms []string, baseStyle, hlStyle lipgloss.Style) string {
	lower := strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			var result strings.Builder
			remaining := text
			for len(remaining) > 0 {
				idx := strings.Index(strings.ToLower(remaining), strings.ToLower(term))
				if idx < 0 {
					result.WriteString(baseStyle.Render(remaining))
					break
				}
				if idx > 0 {
					result.WriteString(baseStyle.Render(remaining[:idx]))
				}
				result.WriteString(hlStyle.Render(remaining[idx : idx+len(term)]))
				remaining = remaining[idx+len(term):]
			}
			return result.String()
		}
	}
	return baseStyle.Render(text)
}

func buildCodeFrame(highlighted, language string, width int) string {
	lines := strings.Split(highlighted, "\n")
	lineCount := len(lines)

	// Remove trailing empty line if present
	if lineCount > 0 && strings.TrimSpace(lines[lineCount-1]) == "" {
		lines = lines[:lineCount-1]
		lineCount = len(lines)
	}

	accentStyle := lipgloss.NewStyle().Foreground(cbAccent)
	lineNumStyle := lipgloss.NewStyle().Foreground(cbMuted)
	dividerStyle := lipgloss.NewStyle().Foreground(cbBorder)

	numWidth := len(fmt.Sprintf("%d", lineCount))
	if numWidth < 2 {
		numWidth = 2
	}

	// Build header
	headerStyle := lipgloss.NewStyle().Foreground(cbHeaderFg).Bold(true)
	langLabel := language
	if langLabel == "" {
		langLabel = "text"
	}
	linesLabel := fmt.Sprintf("%d lines", lineCount)
	if lineCount == 1 {
		linesLabel = "1 line"
	}

	// Inner content width (minus border chrome: 2 for border + 2 for padding)
	innerWidth := width - 4
	if innerWidth < 16 {
		innerWidth = 16
	}

	headerLeft := headerStyle.Render(strings.ToTitle(langLabel[:1]) + langLabel[1:])
	headerRight := lipgloss.NewStyle().Foreground(cbHeaderFg).Render(linesLabel)
	headerPad := innerWidth - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if headerPad < 1 {
		headerPad = 1
	}
	header := headerLeft + strings.Repeat(" ", headerPad) + headerRight

	// Separator line
	sep := lipgloss.NewStyle().Foreground(cbBorder).Render(strings.Repeat("─", innerWidth))

	// Build code lines with accent bar + line numbers
	var codeLines []string
	for i, line := range lines {
		num := fmt.Sprintf("%*d", numWidth, i+1)
		prefix := accentStyle.Render("▎") + " " +
			lineNumStyle.Render(num) + " " +
			dividerStyle.Render("│") + " "
		codeLines = append(codeLines, prefix+line)
	}

	// Compose inner content
	inner := header + "\n" + sep + "\n" + strings.Join(codeLines, "\n")

	// Wrap in bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cbBorder).
		Padding(0, 1).
		Width(width - 2)

	return boxStyle.Render(inner)
}

func renderPlainCodeBlock(code, language string, width int) string {
	// Fallback: wrap raw code in frame without highlighting
	defaultStyle := lipgloss.NewStyle().Foreground(cbFg)
	styledCode := defaultStyle.Render(code)
	return buildCodeFrame(styledCode, language, width)
}

// TopicTerms extracts highlight keywords from a topic slug.
// e.g. "go/interfaces/basics" → ["interface", "interfaces"]
func TopicTerms(slug string) []string {
	if slug == "" {
		return nil
	}

	parts := strings.Split(slug, "/")
	var terms []string
	seen := make(map[string]bool)

	add := func(t string) {
		lower := strings.ToLower(t)
		if !seen[lower] && len(lower) > 2 {
			seen[lower] = true
			terms = append(terms, t)
		}
	}

	// Last segment (the concept)
	if len(parts) >= 1 {
		last := parts[len(parts)-1]
		add(last)
		// Singular form: strip trailing "s"
		if strings.HasSuffix(last, "s") && len(last) > 3 {
			add(last[:len(last)-1])
		}
	}

	// Second-to-last (the category)
	if len(parts) >= 2 {
		cat := parts[len(parts)-2]
		add(cat)
		if strings.HasSuffix(cat, "s") && len(cat) > 3 {
			add(cat[:len(cat)-1])
		}
	}

	return terms
}
