package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// transcriptEntry is the raw JSON structure of one line in a transcript JSONL.
type transcriptEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	Message   json.RawMessage `json:"message"`
}

// transcriptMessage is the message field within a transcript entry.
type transcriptMessage struct {
	Role    string           `json:"role"`
	Content json.RawMessage  `json:"content"`
	Usage   *transcriptUsage `json:"usage,omitempty"`
	Model   string           `json:"model,omitempty"`
}

// transcriptUsage holds the token usage from an assistant message.
type transcriptUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// contentBlock is one element of the assistant message content array.
type contentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// LoadTranscript parses a transcript JSONL file into rounds with blocks in file order.
func LoadTranscript(path string) (*Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := &Transcript{}
	var currentRound *Round
	roundIndex := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		var entry transcriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "user":
			currentRound = handleUserEntry(t, &entry, currentRound, &roundIndex)
		case "assistant":
			if currentRound == nil {
				// Assistant entry before any user message; create a synthetic round.
				currentRound = &Round{Index: roundIndex}
				roundIndex++
				t.Rounds = append(t.Rounds, *currentRound)
			}
			handleAssistantEntry(t, &entry)
		}

		if entry.SessionID != "" && t.SessionID == "" {
			t.SessionID = entry.SessionID
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

// handleUserEntry processes a user-type transcript entry.
// Returns the current round pointer (may be new or existing).
func handleUserEntry(t *Transcript, entry *transcriptEntry, currentRound *Round, roundIndex *int) *Round {
	var msg transcriptMessage
	if entry.Message == nil {
		return currentRound
	}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return currentRound
	}

	// Check if content is a string (real user message) or array (tool_result).
	content := strings.TrimSpace(string(msg.Content))
	if len(content) == 0 {
		return currentRound
	}

	if content[0] == '"' {
		// String content -> new user message, starts a new round.
		var text string
		if err := json.Unmarshal(msg.Content, &text); err != nil {
			return currentRound
		}
		isCtx := isSystemContext(text)
		role := "you"
		if isCtx {
			role = "context"
		}
		r := Round{
			Index:         *roundIndex,
			UserTimestamp:  entry.Timestamp,
			IsContext:      isCtx,
			Blocks:        []Block{{Role: role, Text: text}},
		}
		*roundIndex++
		t.Rounds = append(t.Rounds, r)
		return &t.Rounds[len(t.Rounds)-1]
	}

	// Array content -> likely tool_result, continues current round.
	return currentRound
}

// handleAssistantEntry processes an assistant-type transcript entry,
// appending blocks and usage to the last round.
func handleAssistantEntry(t *Transcript, entry *transcriptEntry) {
	if len(t.Rounds) == 0 {
		return
	}
	r := &t.Rounds[len(t.Rounds)-1]

	var msg transcriptMessage
	if entry.Message == nil {
		return
	}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return
	}

	// Parse content blocks -- each becomes an individual Block in file order.
	content := strings.TrimSpace(string(msg.Content))
	if len(content) > 0 && content[0] == '[' {
		var blocks []contentBlock
		if err := json.Unmarshal(msg.Content, &blocks); err == nil {
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != "" {
						r.Blocks = append(r.Blocks, Block{Role: "claude", Text: b.Text})
					}
				case "thinking":
					if b.Thinking != "" {
						r.Blocks = append(r.Blocks, Block{Role: "thinking", Text: b.Thinking})
					}
				case "tool_use":
					tc := ToolCall{Name: b.Name, InputSummary: toolInputSummary(b.Name, b.Input)}
					r.Blocks = append(r.Blocks, Block{Role: "tool", ToolCall: &tc})
				}
			}
		}
	}

	// Aggregate usage. Output tokens are summed; input/cache tokens take max.
	if msg.Usage != nil {
		r.Usage.OutputTokens += msg.Usage.OutputTokens
		if msg.Usage.InputTokens > r.Usage.InputTokens {
			r.Usage.InputTokens = msg.Usage.InputTokens
		}
		if msg.Usage.CacheCreationInputTokens > r.Usage.CacheCreation {
			r.Usage.CacheCreation = msg.Usage.CacheCreationInputTokens
		}
		if msg.Usage.CacheReadInputTokens > r.Usage.CacheRead {
			r.Usage.CacheRead = msg.Usage.CacheReadInputTokens
		}
	}
}

// isSystemContext detects user messages that are system-injected context
// rather than actual user input.
func isSystemContext(text string) bool {
	return strings.HasPrefix(text, "This session is being continued from a previous conversation") ||
		strings.HasPrefix(text, "Implement the following plan:")
}

// toolInputSummary extracts a short summary from tool input JSON.
func toolInputSummary(toolName string, input json.RawMessage) string {
	if input == nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	switch toolName {
	case "Read":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Write":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Edit":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			if len(cmd) > 80 {
				return cmd[:80] + "..."
			}
			return cmd
		}
	case "Glob":
		if p, ok := m["pattern"].(string); ok {
			return p
		}
	case "Grep":
		if p, ok := m["pattern"].(string); ok {
			return p
		}
	case "Task":
		if d, ok := m["description"].(string); ok {
			return d
		}
	case "WebSearch":
		if q, ok := m["query"].(string); ok {
			return q
		}
	case "WebFetch":
		if u, ok := m["url"].(string); ok {
			return u
		}
	}

	// Fallback: try "command", "file_path", "query", or first string value.
	for _, key := range []string{"command", "file_path", "query", "pattern", "url"} {
		if v, ok := m[key].(string); ok {
			if len(v) > 80 {
				return v[:80] + "..."
			}
			return v
		}
	}

	return fmt.Sprintf("%d params", len(m))
}
