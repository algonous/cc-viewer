package publish

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitLab publishes snippets via the glab CLI.
type GitLab struct{}

func (g *GitLab) Publish(ctx context.Context, s Snippet) (*Result, error) {
	cmd := exec.CommandContext(ctx, "glab", "snippet", "create",
		"--personal",
		"--title", s.Title,
		"--filename", s.Filename,
		"--visibility", "internal",
	)
	cmd.Stdin = strings.NewReader(s.Content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("glab snippet create failed: %s", strings.TrimSpace(string(out)))
	}
	url := parseSnippetURL(strings.TrimSpace(string(out)))
	return &Result{URL: url}, nil
}

// parseSnippetURL extracts the URL from glab's stdout.
// glab typically prints the snippet URL as the last line or a line containing "http".
func parseSnippetURL(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	// Fallback: return the full output trimmed.
	return output
}
