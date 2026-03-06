package data

import (
	"encoding/json"
	"strings"
)

// StreamBlock is one block produced by TranscriptStreamer.
type StreamBlock struct {
	Role     string
	Text     string    // raw text for non-tool blocks
	ToolCall *ToolCall // non-nil for tool blocks
}

// StreamEvent is produced by TranscriptStreamer.ProcessLine for each
// transcript JSONL line that yields visible blocks or usage.
type StreamEvent struct {
	RoundIndex    int
	NewRound      bool
	IsContext     bool
	UserTimestamp string
	Blocks        []StreamBlock
	Usage         *Usage // raw usage from this assistant entry (not aggregated)
}

// TranscriptStreamer processes transcript JSONL lines incrementally,
// maintaining round state across calls. Used by the SSE transcript endpoint.
type TranscriptStreamer struct {
	roundIndex int
	inRound    bool
}

// NewTranscriptStreamer creates a streamer starting at round index 0.
func NewTranscriptStreamer() *TranscriptStreamer {
	return &TranscriptStreamer{}
}

// ProcessLine processes a single transcript JSONL line and returns a
// StreamEvent if the line produces visible blocks or usage. Returns nil
// for lines that produce no visible output (progress, tool_result, etc).
func (s *TranscriptStreamer) ProcessLine(line []byte) *StreamEvent {
	var entry transcriptEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	switch entry.Type {
	case "user":
		return s.processUser(&entry)
	case "assistant":
		return s.processAssistant(&entry)
	}
	return nil
}

// RoundIndex returns the next round index (number of rounds seen so far).
func (s *TranscriptStreamer) RoundIndex() int {
	return s.roundIndex
}

func (s *TranscriptStreamer) processUser(entry *transcriptEntry) *StreamEvent {
	if entry.Message == nil {
		return nil
	}
	var msg transcriptMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil
	}

	content := strings.TrimSpace(string(msg.Content))
	if len(content) == 0 || content[0] != '"' {
		// Array content (tool_result) or empty -- skip.
		return nil
	}

	var text string
	if err := json.Unmarshal(msg.Content, &text); err != nil {
		return nil
	}

	isCtx := isSystemContext(text)
	role := "you"
	if isCtx {
		role = "context"
	}

	ev := &StreamEvent{
		RoundIndex:    s.roundIndex,
		NewRound:      true,
		IsContext:      isCtx,
		UserTimestamp:  entry.Timestamp,
		Blocks:        []StreamBlock{{Role: role, Text: text}},
	}

	s.roundIndex++
	s.inRound = true
	return ev
}

func (s *TranscriptStreamer) processAssistant(entry *transcriptEntry) *StreamEvent {
	if entry.Message == nil {
		return nil
	}
	var msg transcriptMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil
	}

	roundIdx := s.roundIndex - 1
	newRound := false
	if !s.inRound {
		// Assistant entry before any user message; create synthetic round.
		roundIdx = s.roundIndex
		s.roundIndex++
		s.inRound = true
		newRound = true
	}
	if roundIdx < 0 {
		roundIdx = 0
	}

	ev := &StreamEvent{
		RoundIndex: roundIdx,
		NewRound:   newRound,
	}

	// Parse content blocks.
	content := strings.TrimSpace(string(msg.Content))
	if len(content) > 0 && content[0] == '[' {
		var blocks []contentBlock
		if err := json.Unmarshal(msg.Content, &blocks); err == nil {
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != "" {
						ev.Blocks = append(ev.Blocks, StreamBlock{Role: "claude", Text: b.Text})
					}
				case "thinking":
					if b.Thinking != "" {
						ev.Blocks = append(ev.Blocks, StreamBlock{Role: "thinking", Text: b.Thinking})
					}
				case "tool_use":
					tc := &ToolCall{
						Name:         b.Name,
						InputSummary: toolInputSummary(b.Name, b.Input),
						InputJSON:    prettyJSON(b.Input),
					}
					ev.Blocks = append(ev.Blocks, StreamBlock{Role: "tool", ToolCall: tc})
				}
			}
		}
	}

	// Usage (raw, not aggregated -- server handles aggregation).
	if msg.Usage != nil {
		ev.Usage = &Usage{
			InputTokens:   msg.Usage.InputTokens,
			OutputTokens:  msg.Usage.OutputTokens,
			CacheCreation: msg.Usage.CacheCreationInputTokens,
			CacheRead:     msg.Usage.CacheReadInputTokens,
		}
	}

	if len(ev.Blocks) == 0 && ev.Usage == nil {
		return nil
	}

	return ev
}
