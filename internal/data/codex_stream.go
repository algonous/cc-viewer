package data

import (
	"encoding/json"
)

// CodexTranscriptStreamer incrementally parses Codex transcript JSONL.
type CodexTranscriptStreamer struct {
	roundIndex   int
	currentRound int
}

func NewCodexTranscriptStreamer() *CodexTranscriptStreamer {
	return &CodexTranscriptStreamer{currentRound: -1}
}

func (s *CodexTranscriptStreamer) ProcessLine(line []byte) *StreamEvent {
	var entry codexEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	switch entry.Type {
	case "event_msg":
		return s.processEventMsg(&entry)
	case "response_item":
		return s.processResponseItem(&entry)
	default:
		return nil
	}
}

func (s *CodexTranscriptStreamer) processEventMsg(entry *codexEntry) *StreamEvent {
	var ev codexPayloadEvent
	if json.Unmarshal(entry.Payload, &ev) != nil {
		return nil
	}

	switch ev.Type {
	case "user_message":
		round := s.roundIndex
		s.currentRound = round
		s.roundIndex++
		isCtx := isSystemContext(ev.Message)
		role := "you"
		if isCtx {
			role = "context"
		}
		return &StreamEvent{
			RoundIndex:    round,
			NewRound:      true,
			IsContext:     isCtx,
			UserTimestamp: entry.Timestamp,
			Blocks: []StreamBlock{{
				Role: role,
				Text: ev.Message,
			}},
		}
	case "agent_message":
		round := s.ensureRound()
		if ev.Message == "" {
			return nil
		}
		return &StreamEvent{
			RoundIndex: round,
			Blocks: []StreamBlock{{
				Role: "claude",
				Text: ev.Message,
			}},
		}
	case "token_count":
		round := s.ensureRound()
		var tc codexPayloadTokenCount
		if json.Unmarshal(entry.Payload, &tc) != nil {
			return nil
		}
		last := tc.Info.LastTokenUsage
		return &StreamEvent{
			RoundIndex: round,
			Usage: &Usage{
				InputTokens:  last.InputTokens,
				OutputTokens: last.OutputTokens,
				CacheRead:    last.CachedInputTokens,
			},
		}
	default:
		return nil
	}
}

func (s *CodexTranscriptStreamer) processResponseItem(entry *codexEntry) *StreamEvent {
	round := s.ensureRound()
	var ri codexPayloadResponseItem
	if json.Unmarshal(entry.Payload, &ri) != nil {
		return nil
	}

	switch ri.Type {
	case "function_call":
		tc := &ToolCall{
			Name:         ri.Name,
			InputSummary: codexToolInputSummary(ri.Name, ri.Arguments),
			InputJSON:    normalizeRawJSON(ri.Arguments),
		}
		return &StreamEvent{RoundIndex: round, Blocks: []StreamBlock{{Role: "tool", ToolCall: tc}}}
	case "custom_tool_call":
		tc := &ToolCall{
			Name:         ri.Name,
			InputSummary: codexToolInputSummary(ri.Name, ri.Input),
			InputJSON:    normalizeRawJSON(ri.Input),
		}
		return &StreamEvent{RoundIndex: round, Blocks: []StreamBlock{{Role: "tool", ToolCall: tc}}}
	default:
		return nil
	}
}

func (s *CodexTranscriptStreamer) ensureRound() int {
	if s.currentRound >= 0 {
		return s.currentRound
	}
	round := s.roundIndex
	s.currentRound = round
	s.roundIndex++
	return round
}
