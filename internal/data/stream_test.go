package data

import (
	"testing"
)

func TestTranscriptStreamerBasic(t *testing.T) {
	s := NewTranscriptStreamer()

	// User message -> new round.
	ev := s.ProcessLine([]byte(`{"type":"user","timestamp":"2026-02-26T11:00:00Z","sessionId":"s1","message":{"role":"user","content":"hello there"}}`))
	if ev == nil {
		t.Fatal("expected event for user message")
	}
	if ev.RoundIndex != 0 || !ev.NewRound {
		t.Errorf("round=%d new=%v, want 0/true", ev.RoundIndex, ev.NewRound)
	}
	if len(ev.Blocks) != 1 || ev.Blocks[0].Role != "you" || ev.Blocks[0].Text != "hello there" {
		t.Errorf("blocks=%+v", ev.Blocks)
	}
	if ev.IsContext {
		t.Error("should not be context")
	}

	// Assistant with thinking + text + tool_use.
	ev = s.ProcessLine([]byte(`{"type":"assistant","timestamp":"2026-02-26T11:00:05Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Hi!"},{"type":"tool_use","name":"Read","input":{"file_path":"/foo.go"}}],"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":200}}}`))
	if ev == nil {
		t.Fatal("expected event for assistant message")
	}
	if ev.RoundIndex != 0 || ev.NewRound {
		t.Errorf("round=%d new=%v, want 0/false", ev.RoundIndex, ev.NewRound)
	}
	if len(ev.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(ev.Blocks))
	}
	if ev.Blocks[0].Role != "thinking" || ev.Blocks[0].Text != "hmm" {
		t.Errorf("block 0: %+v", ev.Blocks[0])
	}
	if ev.Blocks[1].Role != "claude" || ev.Blocks[1].Text != "Hi!" {
		t.Errorf("block 1: %+v", ev.Blocks[1])
	}
	if ev.Blocks[2].Role != "tool" || ev.Blocks[2].ToolCall == nil || ev.Blocks[2].ToolCall.Name != "Read" {
		t.Errorf("block 2: %+v", ev.Blocks[2])
	}
	if ev.Usage == nil || ev.Usage.OutputTokens != 50 || ev.Usage.InputTokens != 100 {
		t.Errorf("usage: %+v", ev.Usage)
	}

	// Second user message -> new round.
	ev = s.ProcessLine([]byte(`{"type":"user","timestamp":"2026-02-26T11:01:00Z","sessionId":"s1","message":{"role":"user","content":"fix bug"}}`))
	if ev == nil {
		t.Fatal("expected event for second user message")
	}
	if ev.RoundIndex != 1 || !ev.NewRound {
		t.Errorf("round=%d new=%v, want 1/true", ev.RoundIndex, ev.NewRound)
	}
}

func TestTranscriptStreamerToolResult(t *testing.T) {
	s := NewTranscriptStreamer()

	// User message.
	s.ProcessLine([]byte(`{"type":"user","timestamp":"2026-02-26T11:00:00Z","sessionId":"s1","message":{"role":"user","content":"hello"}}`))

	// Tool result (array content) should be skipped.
	ev := s.ProcessLine([]byte(`{"type":"user","timestamp":"2026-02-26T11:00:10Z","sessionId":"s1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"123","content":"file"}]}}`))
	if ev != nil {
		t.Error("tool_result should return nil")
	}
}

func TestTranscriptStreamerContext(t *testing.T) {
	s := NewTranscriptStreamer()

	ev := s.ProcessLine([]byte(`{"type":"user","timestamp":"2026-02-26T11:00:00Z","sessionId":"s1","message":{"role":"user","content":"Implement the following plan:\n\n# My Plan"}}`))
	if ev == nil {
		t.Fatal("expected event")
	}
	if !ev.IsContext || ev.Blocks[0].Role != "context" {
		t.Errorf("context=%v role=%s", ev.IsContext, ev.Blocks[0].Role)
	}
}

func TestTranscriptStreamerProgress(t *testing.T) {
	s := NewTranscriptStreamer()

	// Progress entry should be skipped.
	ev := s.ProcessLine([]byte(`{"type":"progress","sessionId":"s1"}`))
	if ev != nil {
		t.Error("progress should return nil")
	}
}

func TestTranscriptStreamerSyntheticRound(t *testing.T) {
	s := NewTranscriptStreamer()

	// Assistant before any user message -> synthetic round.
	ev := s.ProcessLine([]byte(`{"type":"assistant","timestamp":"2026-02-26T11:00:00Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}],"usage":{"input_tokens":10,"output_tokens":5}}}`))
	if ev == nil {
		t.Fatal("expected event for synthetic round")
	}
	if ev.RoundIndex != 0 || !ev.NewRound {
		t.Errorf("round=%d new=%v, want 0/true", ev.RoundIndex, ev.NewRound)
	}
}

func TestParseHistoryLine(t *testing.T) {
	line := []byte(`{"sessionId":"aaa","timestamp":1000,"project":"/Users/kfu/code/foo","display":"hello world"}`)
	u := ParseHistoryLine(line)
	if u == nil {
		t.Fatal("expected non-nil")
	}
	if u.SessionID != "claude:aaa" || u.Timestamp != 1000 || u.Display != "hello world" {
		t.Errorf("got %+v", u)
	}
	if u.ProjectName != "foo" {
		t.Errorf("project name = %q", u.ProjectName)
	}
}

func TestParseHistoryLineInvalid(t *testing.T) {
	if ParseHistoryLine([]byte(`not json`)) != nil {
		t.Error("should be nil for invalid json")
	}
	if ParseHistoryLine([]byte(`{"timestamp":1000}`)) != nil {
		t.Error("should be nil for missing sessionId")
	}
}
