package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExportSession writes a session transcript as structured JSONL to the export directory.
// Returns the path of the written file.
func ExportSession(configDir string, session SessionSummary, transcript *Transcript) (string, error) {
	return ExportSessionRounds(configDir, session, transcript, nil, true)
}

// ExportSessionRounds writes selected rounds as JSONL. If indices is nil, all rounds are exported.
func ExportSessionRounds(configDir string, session SessionSummary, transcript *Transcript, indices []int, includeThinking bool) (string, error) {
	exportDir := filepath.Join(configDir, "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(exportDir, session.SessionID+".jsonl")
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	rounds := selectRounds(transcript.Rounds, indices)
	for _, r := range rounds {
		tools := make([]ExportTool, len(r.ToolCalls))
		for i, tc := range r.ToolCalls {
			tools[i] = ExportTool{
				Name:         tc.Name,
				InputSummary: tc.InputSummary,
			}
		}

		er := ExportRound{
			SessionID:         session.SessionID,
			Timestamp:         r.UserTimestamp,
			Project:           session.Project,
			RoundIndex:        r.Index,
			IsContext:         r.IsContext,
			UserMessage:       r.UserMessage,
			ToolCalls:         tools,
			AssistantResponse: strings.Join(r.AssistantTexts, "\n"),
			Usage: ExportUsage{
				InputTokens:   r.Usage.InputTokens,
				OutputTokens:  r.Usage.OutputTokens,
				CacheRead:     r.Usage.CacheRead,
				CacheCreation: r.Usage.CacheCreation,
			},
		}
		if includeThinking && len(r.ThinkingTexts) > 0 {
			er.ThinkingTexts = r.ThinkingTexts
		}
		if err := enc.Encode(er); err != nil {
			return "", err
		}
	}

	return outPath, nil
}

// ExportSessionMarkdown writes a session transcript as markdown.
// Returns the path of the written file.
func ExportSessionMarkdown(configDir string, session SessionSummary, transcript *Transcript, indices []int, includeThinking bool) (string, error) {
	exportDir := filepath.Join(configDir, "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(exportDir, session.SessionID+".md")
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b := GenerateMarkdown(session, transcript, indices, nil, includeThinking)
	if _, err := f.Write(b); err != nil {
		return "", err
	}

	return outPath, nil
}

// GenerateJSONL returns JSONL bytes for the given session/transcript.
// When blockRoles is non-nil, it specifies which block types to include per round index.
func GenerateJSONL(session SessionSummary, transcript *Transcript, indices []int, blockRoles map[int][]string, includeThinking bool) []byte {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	rounds := selectRoundsWithBlockRoles(transcript.Rounds, indices, blockRoles)
	for _, r := range rounds {
		roles := blockRoles[r.Index]

		tools := make([]ExportTool, len(r.ToolCalls))
		for i, tc := range r.ToolCalls {
			tools[i] = ExportTool{Name: tc.Name, InputSummary: tc.InputSummary}
		}
		er := ExportRound{
			SessionID:  session.SessionID,
			Timestamp:  r.UserTimestamp,
			Project:    session.Project,
			RoundIndex: r.Index,
			IsContext:  r.IsContext,
			Usage: ExportUsage{
				InputTokens:   r.Usage.InputTokens,
				OutputTokens:  r.Usage.OutputTokens,
				CacheRead:     r.Usage.CacheRead,
				CacheCreation: r.Usage.CacheCreation,
			},
		}
		if shouldIncludeRole(roles, "you", "context") {
			er.UserMessage = r.UserMessage
		}
		if shouldIncludeRole(roles, "tool") {
			er.ToolCalls = tools
		}
		if includeThinking && len(r.ThinkingTexts) > 0 && shouldIncludeRole(roles, "thinking") {
			er.ThinkingTexts = r.ThinkingTexts
		}
		if shouldIncludeRole(roles, "claude") {
			er.AssistantResponse = strings.Join(r.AssistantTexts, "\n")
		}
		enc.Encode(er)
	}
	return []byte(buf.String())
}

// GenerateMarkdown returns markdown bytes for the given session/transcript.
// When blockRoles is non-nil, it specifies which block types to include per round index.
func GenerateMarkdown(session SessionSummary, transcript *Transcript, indices []int, blockRoles map[int][]string, includeThinking bool) []byte {
	var buf strings.Builder
	rounds := selectRoundsWithBlockRoles(transcript.Rounds, indices, blockRoles)

	var totalUsage Usage
	for _, r := range rounds {
		totalUsage.InputTokens += r.Usage.InputTokens
		totalUsage.OutputTokens += r.Usage.OutputTokens
		totalUsage.CacheCreation += r.Usage.CacheCreation
		totalUsage.CacheRead += r.Usage.CacheRead
	}

	fmt.Fprintf(&buf, "# Session %s\n\n", session.SessionID)
	fmt.Fprintf(&buf, "- **Rounds**: %d\n", len(rounds))
	fmt.Fprintf(&buf, "- **Total tokens**: in=%d out=%d cache_read=%d cache_write=%d\n\n",
		totalUsage.InputTokens, totalUsage.OutputTokens, totalUsage.CacheRead, totalUsage.CacheCreation)
	fmt.Fprintf(&buf, "---\n\n")

	for _, r := range rounds {
		roles := blockRoles[r.Index]
		ts := r.UserTimestamp
		if ts == "" {
			ts = "unknown"
		}
		fmt.Fprintf(&buf, "## Round %d (%s)\n\n", r.Index+1, ts)

		if r.UserMessage != "" && shouldIncludeRole(roles, "you", "context") {
			fmt.Fprintf(&buf, "```prompt\n%s\n```\n\n", r.UserMessage)
		}

		if len(r.ToolCalls) > 0 && shouldIncludeRole(roles, "tool") {
			fmt.Fprintf(&buf, "```tool_use\n")
			for _, tc := range r.ToolCalls {
				if tc.InputSummary != "" {
					fmt.Fprintf(&buf, "%s: %s\n", tc.Name, tc.InputSummary)
				} else {
					fmt.Fprintf(&buf, "%s\n", tc.Name)
				}
			}
			fmt.Fprintf(&buf, "```\n\n")
		}

		if includeThinking && len(r.ThinkingTexts) > 0 && shouldIncludeRole(roles, "thinking") {
			fmt.Fprintf(&buf, "```thinking\n%s\n```\n\n", strings.Join(r.ThinkingTexts, "\n\n"))
		}

		if len(r.AssistantTexts) > 0 && shouldIncludeRole(roles, "claude") {
			fmt.Fprintf(&buf, "```assistant\n%s\n```\n\n", strings.Join(r.AssistantTexts, "\n"))
		}

		fmt.Fprintf(&buf, "> Tokens: in=%d out=%d cache_read=%d cache_write=%d\n\n",
			r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheRead, r.Usage.CacheCreation)
	}

	return []byte(buf.String())
}

// selectRounds returns the subset of rounds at the given indices, or all rounds if indices is nil.
func selectRounds(rounds []Round, indices []int) []Round {
	if indices == nil {
		return rounds
	}
	set := make(map[int]bool, len(indices))
	for _, i := range indices {
		set[i] = true
	}
	var result []Round
	for _, r := range rounds {
		if set[r.Index] {
			result = append(result, r)
		}
	}
	return result
}

// selectRoundsWithBlockRoles returns the subset of rounds to export.
// When blockRoles is non-nil, it determines which rounds to include (by round index key).
// Otherwise falls back to selectRounds with indices.
func selectRoundsWithBlockRoles(rounds []Round, indices []int, blockRoles map[int][]string) []Round {
	if blockRoles != nil {
		var result []Round
		for _, r := range rounds {
			if _, ok := blockRoles[r.Index]; ok {
				result = append(result, r)
			}
		}
		return result
	}
	return selectRounds(rounds, indices)
}

// shouldIncludeRole returns true if the given role (or any of the alternates) should be included.
// When roles is nil (no block-level filter for this round), all roles are included.
func shouldIncludeRole(roles []string, names ...string) bool {
	if roles == nil {
		return true
	}
	for _, r := range roles {
		for _, n := range names {
			if r == n {
				return true
			}
		}
	}
	return false
}

// ConfigDir returns the cc-viewer config directory, respecting XDG_CONFIG_HOME.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cc-viewer")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cc-viewer")
}
