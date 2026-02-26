package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ExportSession writes a session transcript as structured JSONL to the export directory.
// Returns the path of the written file.
func ExportSession(configDir string, session SessionSummary, transcript *Transcript) (string, error) {
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

	for _, r := range transcript.Rounds {
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
		if err := enc.Encode(er); err != nil {
			return "", err
		}
	}

	return outPath, nil
}

// ConfigDir returns the cc-tree config directory, respecting XDG_CONFIG_HOME.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cc-tree")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cc-tree")
}
