package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_InlineMathPlaceholder(t *testing.T) {
	r := New()
	html := r.RenderMarkdown("这是公式 $x^2 + 1$。")
	if !strings.Contains(html, `class="katex"`) {
		t.Fatalf("expected katex output, got: %s", html)
	}
	if strings.Contains(html, "$x^2 + 1$") {
		t.Fatalf("expected rendered inline math, got raw tex: %s", html)
	}
}

func TestRenderMarkdown_IgnoresCurrency(t *testing.T) {
	r := New()
	html := r.RenderMarkdown("价格是 $5，不是公式。")
	if strings.Contains(html, `class="math-placeholder"`) {
		t.Fatalf("unexpected math placeholder for currency: %s", html)
	}
	if !strings.Contains(html, "$5") {
		t.Fatalf("expected currency text preserved: %s", html)
	}
}

func TestRenderMarkdown_DoesNotParseCodeFenceAsMath(t *testing.T) {
	r := New()
	input := "```\nconst value = '$x$'\n```"
	html := r.RenderMarkdown(input)
	if strings.Contains(html, `class="katex"`) {
		t.Fatalf("unexpected katex output in code fence: %s", html)
	}
}

func TestRenderMarkdown_DisplayMathPlaceholder(t *testing.T) {
	r := New()
	html := r.RenderMarkdown("$$\\frac{1}{2}$$")
	if !strings.Contains(html, `katex-display`) {
		t.Fatalf("expected display katex output, got: %s", html)
	}
	if strings.Contains(html, "$$\\frac{1}{2}$$") {
		t.Fatalf("expected rendered display math, got raw tex: %s", html)
	}
}

func TestRenderMarkdown_PrimeAndMultiInline(t *testing.T) {
	r := New()
	html := r.RenderMarkdown("例如：$(x^2)'=2x$，$(\\sqrt{x})' = \\frac{1}{2\\sqrt{x}}$。")
	if strings.Count(html, `class="katex"`) < 2 {
		t.Fatalf("expected 2 rendered math segments, got: %s", html)
	}
}

func TestRenderMarkdown_BracketDelimiters(t *testing.T) {
	r := New()
	html := r.RenderMarkdown("\\(x+y\\) and \\[x^2+y^2\\]")
	if strings.Count(html, `class="katex"`) < 2 {
		t.Fatalf("expected rendered bracket math segments, got: %s", html)
	}
}
