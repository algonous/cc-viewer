package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTranscript(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"file-history-snapshot","snapshot":{}}
{"type":"user","timestamp":"2026-02-26T11:00:00Z","sessionId":"sess1","message":{"role":"user","content":"hello there"}}
{"type":"assistant","timestamp":"2026-02-26T11:00:05Z","sessionId":"sess1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Hi! How can I help?"}],"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":200}}}
{"type":"assistant","timestamp":"2026-02-26T11:00:06Z","sessionId":"sess1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo/bar.go"}}],"usage":{"input_tokens":100,"output_tokens":30,"cache_creation_input_tokens":10,"cache_read_input_tokens":200}}}
{"type":"progress","sessionId":"sess1"}
{"type":"user","timestamp":"2026-02-26T11:00:10Z","sessionId":"sess1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"123","content":"file contents here"}]}}
{"type":"assistant","timestamp":"2026-02-26T11:00:15Z","sessionId":"sess1","message":{"role":"assistant","content":[{"type":"text","text":"The file contains some Go code."}],"usage":{"input_tokens":150,"output_tokens":40,"cache_creation_input_tokens":20,"cache_read_input_tokens":300}}}
{"type":"user","timestamp":"2026-02-26T11:01:00Z","sessionId":"sess1","message":{"role":"user","content":"now fix the bug"}}
{"type":"assistant","timestamp":"2026-02-26T11:01:05Z","sessionId":"sess1","message":{"role":"assistant","content":[{"type":"text","text":"I will fix the bug now."}],"usage":{"input_tokens":200,"output_tokens":60,"cache_creation_input_tokens":30,"cache_read_input_tokens":400}}}
`
	path := filepath.Join(dir, "sess1.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tr, err := LoadTranscript(path)
	if err != nil {
		t.Fatal(err)
	}

	if tr.SessionID != "sess1" {
		t.Errorf("expected session ID sess1, got %q", tr.SessionID)
	}

	if len(tr.Rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(tr.Rounds))
	}

	// Round 0: blocks in file order.
	r0 := tr.Rounds[0]
	if r0.IsContext {
		t.Error("round 0 should not be context")
	}
	if r0.UserTimestamp != "2026-02-26T11:00:00Z" {
		t.Errorf("round 0 timestamp = %q", r0.UserTimestamp)
	}

	// Round 0 should have 5 blocks: you, thinking, claude, tool, claude
	if len(r0.Blocks) != 5 {
		t.Fatalf("round 0 expected 5 blocks, got %d", len(r0.Blocks))
	}

	// Block 0: you
	if r0.Blocks[0].Role != "you" || r0.Blocks[0].Text != "hello there" {
		t.Errorf("block 0: role=%q text=%q", r0.Blocks[0].Role, r0.Blocks[0].Text)
	}

	// Block 1: thinking
	if r0.Blocks[1].Role != "thinking" || r0.Blocks[1].Text != "hmm" {
		t.Errorf("block 1: role=%q text=%q", r0.Blocks[1].Role, r0.Blocks[1].Text)
	}

	// Block 2: claude
	if r0.Blocks[2].Role != "claude" || r0.Blocks[2].Text != "Hi! How can I help?" {
		t.Errorf("block 2: role=%q text=%q", r0.Blocks[2].Role, r0.Blocks[2].Text)
	}

	// Block 3: tool
	if r0.Blocks[3].Role != "tool" || r0.Blocks[3].ToolCall == nil {
		t.Fatalf("block 3: role=%q toolcall=%v", r0.Blocks[3].Role, r0.Blocks[3].ToolCall)
	}
	if r0.Blocks[3].ToolCall.Name != "Read" || r0.Blocks[3].ToolCall.InputSummary != "/foo/bar.go" {
		t.Errorf("block 3 tool: name=%q input=%q", r0.Blocks[3].ToolCall.Name, r0.Blocks[3].ToolCall.InputSummary)
	}

	// Block 4: claude (after tool_result)
	if r0.Blocks[4].Role != "claude" || r0.Blocks[4].Text != "The file contains some Go code." {
		t.Errorf("block 4: role=%q text=%q", r0.Blocks[4].Role, r0.Blocks[4].Text)
	}

	// Usage: output_tokens summed (50+30+40=120), input max (150).
	if r0.Usage.OutputTokens != 120 {
		t.Errorf("round 0 output tokens = %d, want 120", r0.Usage.OutputTokens)
	}
	if r0.Usage.InputTokens != 150 {
		t.Errorf("round 0 input tokens = %d, want 150", r0.Usage.InputTokens)
	}
	if r0.Usage.CacheRead != 300 {
		t.Errorf("round 0 cache read = %d, want 300", r0.Usage.CacheRead)
	}

	// Round 1: 2 blocks (you, claude).
	r1 := tr.Rounds[1]
	if len(r1.Blocks) != 2 {
		t.Fatalf("round 1 expected 2 blocks, got %d", len(r1.Blocks))
	}
	if r1.Blocks[0].Role != "you" || r1.Blocks[0].Text != "now fix the bug" {
		t.Errorf("round 1 block 0: role=%q text=%q", r1.Blocks[0].Role, r1.Blocks[0].Text)
	}
	if r1.Blocks[1].Role != "claude" || r1.Blocks[1].Text != "I will fix the bug now." {
		t.Errorf("round 1 block 1: role=%q text=%q", r1.Blocks[1].Role, r1.Blocks[1].Text)
	}
	if r1.Usage.OutputTokens != 60 {
		t.Errorf("round 1 output tokens = %d, want 60", r1.Usage.OutputTokens)
	}
}

func TestLoadTranscriptContext(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"user","timestamp":"2026-02-26T11:00:00Z","sessionId":"s1","message":{"role":"user","content":"Implement the following plan:\n\n# My Plan\n\nDo stuff"}}
{"type":"assistant","timestamp":"2026-02-26T11:00:05Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"OK"}],"usage":{"input_tokens":10,"output_tokens":5}}}
{"type":"user","timestamp":"2026-02-26T11:01:00Z","sessionId":"s1","message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context."}}
{"type":"assistant","timestamp":"2026-02-26T11:01:05Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"Got it"}],"usage":{"input_tokens":10,"output_tokens":5}}}
{"type":"user","timestamp":"2026-02-26T11:02:00Z","sessionId":"s1","message":{"role":"user","content":"now do the real thing"}}
{"type":"assistant","timestamp":"2026-02-26T11:02:05Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"Done"}],"usage":{"input_tokens":10,"output_tokens":5}}}
`
	path := filepath.Join(dir, "ctx.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(tr.Rounds))
	}

	// Round 0: plan impl -> context block.
	if !tr.Rounds[0].IsContext {
		t.Error("round 0 (plan impl) should be context")
	}
	if tr.Rounds[0].Blocks[0].Role != "context" {
		t.Errorf("round 0 block 0 role = %q, want context", tr.Rounds[0].Blocks[0].Role)
	}

	// Round 1: continuation -> context block.
	if !tr.Rounds[1].IsContext {
		t.Error("round 1 (continuation) should be context")
	}
	if tr.Rounds[1].Blocks[0].Role != "context" {
		t.Errorf("round 1 block 0 role = %q, want context", tr.Rounds[1].Blocks[0].Role)
	}

	// Round 2: real user message -> you block.
	if tr.Rounds[2].IsContext {
		t.Error("round 2 (real user msg) should not be context")
	}
	if tr.Rounds[2].Blocks[0].Role != "you" {
		t.Errorf("round 2 block 0 role = %q, want you", tr.Rounds[2].Blocks[0].Role)
	}
}

func TestLoadTranscriptEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Rounds) != 0 {
		t.Errorf("expected 0 rounds, got %d", len(tr.Rounds))
	}
}

func TestToolInputSummary(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"Read", "Read", `{"file_path":"/foo/bar.go"}`, "/foo/bar.go"},
		{"Bash", "Bash", `{"command":"ls -la"}`, "ls -la"},
		{"Bash long", "Bash", `{"command":"` + strings.Repeat("x", 100) + `"}`, strings.Repeat("x", 80) + "..."},
		{"Grep", "Grep", `{"pattern":"foo.*bar"}`, "foo.*bar"},
		{"Unknown", "Custom", `{"a":1,"b":2}`, "2 params"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolInputSummary(tt.tool, []byte(tt.input))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
