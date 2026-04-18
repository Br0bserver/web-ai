package render

import (
	"bytes"
	stdhtml "html"
	"strconv"
	"strings"
	"unicode"

	katex "github.com/FurqanSoftware/goldmark-katex"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	renderhtml "github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

type mathSegment struct {
	Tex     string
	Display bool
}

func New() *Renderer {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class").OnElements("code", "pre", "table", "th", "td")
	policy.AllowElements("table", "thead", "tbody", "tr", "th", "td")

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table, extension.Strikethrough),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(renderhtml.WithHardWraps(), renderhtml.WithXHTML()),
	)

	return &Renderer{markdown: md, policy: policy}
}

func (r *Renderer) RenderMarkdown(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	protected, mathSegments := protectMathSegments(trimmed)
	var buffer bytes.Buffer
	if err := r.markdown.Convert([]byte(protected), &buffer); err != nil {
		return "<p>" + stdhtml.EscapeString(trimmed) + "</p>"
	}
	html := r.policy.Sanitize(buffer.String())
	return restoreMathSegments(html, mathSegments)
}

func protectMathSegments(input string) (string, []mathSegment) {
	segments := []mathSegment{}
	var builder strings.Builder
	inFence := false
	for i := 0; i < len(input); {
		if strings.HasPrefix(input[i:], "```") {
			inFence = !inFence
			builder.WriteString("```")
			i += 3
			continue
		}
		if inFence {
			builder.WriteByte(input[i])
			i++
			continue
		}
		if input[i] == '`' {
			if end := strings.IndexByte(input[i+1:], '`'); end >= 0 {
				builder.WriteString(input[i : i+1+end+1])
				i += end + 2
				continue
			}
		}
		if strings.HasPrefix(input[i:], "$$") {
			if end := findUnescaped(input, "$$", i+2); end >= 0 && end > i+2 {
				tex := strings.TrimSpace(input[i+2 : end])
				token := mathToken(len(segments))
				segments = append(segments, mathSegment{Tex: tex, Display: true})
				builder.WriteString(token)
				i = end + 2
				continue
			}
		}
		if strings.HasPrefix(input[i:], `\[`) {
			if end := findEscapedDelimiter(input, i+2, `\]`); end >= 0 && end > i+2 {
				tex := strings.TrimSpace(input[i+2 : end])
				token := mathToken(len(segments))
				segments = append(segments, mathSegment{Tex: tex, Display: true})
				builder.WriteString(token)
				i = end + 2
				continue
			}
		}
		if strings.HasPrefix(input[i:], `\(`) {
			if end := findEscapedDelimiter(input, i+2, `\)`); end >= 0 && end > i+2 {
				tex := strings.TrimSpace(input[i+2 : end])
				token := mathToken(len(segments))
				segments = append(segments, mathSegment{Tex: tex, Display: false})
				builder.WriteString(token)
				i = end + 2
				continue
			}
		}
		if isInlineDollarStart(input, i) {
			if end := findInlineDollarEnd(input, i+1); end > i+1 {
				tex := strings.TrimSpace(input[i+1 : end])
				if !looksLikeMath(tex) {
					builder.WriteByte(input[i])
					i++
					continue
				}
				token := mathToken(len(segments))
				segments = append(segments, mathSegment{Tex: tex, Display: false})
				builder.WriteString(token)
				i = end + 1
				continue
			}
		}
		builder.WriteByte(input[i])
		i++
	}
	return builder.String(), segments
}

func restoreMathSegments(html string, segments []mathSegment) string {
	for i := 0; i < len(segments); i++ {
		html = strings.ReplaceAll(html, mathToken(i), renderMathSegment(segments[i]))
	}
	return html
}

func renderMathSegment(segment mathSegment) string {
	var buffer bytes.Buffer
	if err := katex.Render(&buffer, []byte(segment.Tex), segment.Display, false); err != nil {
		return fallbackMathSegment(segment)
	}
	rendered := strings.TrimSpace(buffer.String())
	if rendered == "" {
		return fallbackMathSegment(segment)
	}
	return rendered
}

func fallbackMathSegment(segment mathSegment) string {
	if segment.Display {
		return `<div class="math-fallback">` + stdhtml.EscapeString("$$"+segment.Tex+"$$") + `</div>`
	}
	return `<span class="math-fallback">` + stdhtml.EscapeString("$"+segment.Tex+"$") + `</span>`
}

func mathToken(index int) string {
	return "@@MATH_SEG_" + strconv.Itoa(index) + "@@"
}

func findUnescaped(input, needle string, start int) int {
	for i := start; i <= len(input)-len(needle); i++ {
		if strings.HasPrefix(input[i:], needle) && !isEscaped(input, i) {
			return i
		}
	}
	return -1
}

func findEscapedDelimiter(input string, start int, delimiter string) int {
	for i := start; i <= len(input)-len(delimiter); i++ {
		if strings.HasPrefix(input[i:], delimiter) && !isEscaped(input, i) {
			return i
		}
	}
	return -1
}

func isEscaped(input string, index int) bool {
	if index <= 0 || index > len(input) {
		return false
	}
	count := 0
	for i := index - 1; i >= 0 && input[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func isInlineDollarStart(input string, index int) bool {
	if index < 0 || index >= len(input) || input[index] != '$' || isEscaped(input, index) {
		return false
	}
	if strings.HasPrefix(input[index:], "$$") {
		return false
	}
	if index+1 >= len(input) {
		return false
	}
	next := rune(input[index+1])
	if next == '$' || unicode.IsSpace(next) {
		return false
	}
	if index > 0 {
		prev := rune(input[index-1])
		if unicode.IsDigit(prev) {
			return false
		}
		if unicode.IsLetter(prev) && unicode.IsLetter(next) {
			return false
		}
	}
	return true
}

func findInlineDollarEnd(input string, start int) int {
	for i := start; i < len(input); i++ {
		if input[i] == '\n' {
			return -1
		}
		if input[i] != '$' || isEscaped(input, i) {
			continue
		}
		if i+1 < len(input) && input[i+1] == '$' {
			continue
		}
		if unicode.IsSpace(rune(input[i-1])) {
			continue
		}
		if i+1 < len(input) && unicode.IsDigit(rune(input[i+1])) {
			continue
		}
		return i
	}
	return -1
}

func looksLikeMath(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "\\") {
		return true
	}
	if strings.IndexAny(trimmed, "^_{}=+-*/<>[]()") >= 0 {
		return true
	}
	if utf8Len(trimmed) <= 3 {
		for _, r := range trimmed {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return true
			}
		}
	}
	return false
}

func utf8Len(s string) int {
	return len([]rune(s))
}
