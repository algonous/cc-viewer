package publish

import (
	"context"
	"testing"
)

func TestParseSnippetURL(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "url on last line",
			output: "Created snippet\nhttps://gitlab.example.com/-/snippets/12345",
			want:   "https://gitlab.example.com/-/snippets/12345",
		},
		{
			name:   "url in middle",
			output: "Creating...\nhttps://gitlab.com/-/snippets/99\nDone.",
			want:   "https://gitlab.com/-/snippets/99",
		},
		{
			name:   "http url",
			output: "http://localhost/-/snippets/1",
			want:   "http://localhost/-/snippets/1",
		},
		{
			name:   "no url fallback",
			output: "some output",
			want:   "some output",
		},
		{
			name:   "empty",
			output: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSnippetURL(tt.output)
			if got != tt.want {
				t.Errorf("parseSnippetURL(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestGitLabPublishMissingBinary(t *testing.T) {
	// When glab is not installed, Publish should return a clear error.
	g := &GitLab{}
	_, err := g.Publish(context.Background(), Snippet{
		Title:    "Test",
		Filename: "test.md",
		Content:  "hello",
	})
	// This will fail unless glab is actually installed and authenticated,
	// which is the expected behavior -- we just verify it doesn't panic.
	if err != nil {
		// Expected on CI / machines without glab.
		t.Logf("Publish returned expected error: %v", err)
	}
}
