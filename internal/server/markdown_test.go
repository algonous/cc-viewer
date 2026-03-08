package server

import (
	"strings"
	"testing"
)

func TestRenderMarkdownMixedLists(t *testing.T) {
	in := `1. Ingest

- Input EPUB
- Parse chapters/paragraphs/sentences

2. User model

- Keep a per-user profile`

	html := renderMarkdown(in)
	if strings.Count(html, "<ol>") != 1 {
		t.Fatalf("expected a single ordered list, got:\n%s", html)
	}
	if !strings.Contains(html, "<li>Ingest\n<ul>") {
		t.Fatalf("expected bullets nested under first ordered item, got:\n%s", html)
	}
	if !strings.Contains(html, "</ul>\n</li>\n<li>User model\n<ul>") {
		t.Fatalf("expected second ordered item with nested bullets, got:\n%s", html)
	}
}
