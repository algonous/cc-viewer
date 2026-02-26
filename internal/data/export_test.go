package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportSession(t *testing.T) {
	configDir := t.TempDir()

	session := SessionSummary{
		SessionID:   "test-session",
		Project:     "/Users/kfu/code/foo",
		ProjectName: "foo",
	}

	transcript := &Transcript{
		SessionID: "test-session",
		Rounds: []Round{
			{
				Index:          0,
				UserMessage:    "hello",
				UserTimestamp:   "2026-02-26T11:00:00Z",
				AssistantTexts: []string{"Hi there!", "How can I help?"},
				ThinkingTexts:  []string{"let me think about this"},
				ToolCalls: []ToolCall{
					{Name: "Read", InputSummary: "/foo/bar.go"},
				},
				Usage: Usage{InputTokens: 100, OutputTokens: 50, CacheRead: 200, CacheCreation: 10},
			},
			{
				Index:          1,
				UserMessage:    "fix bug",
				UserTimestamp:   "2026-02-26T11:01:00Z",
				AssistantTexts: []string{"Done!"},
				Usage:          Usage{InputTokens: 200, OutputTokens: 60},
			},
		},
	}

	outPath, err := ExportSession(configDir, session, transcript)
	if err != nil {
		t.Fatal(err)
	}

	expectedPath := filepath.Join(configDir, "exports", "test-session.jsonl")
	if outPath != expectedPath {
		t.Errorf("output path = %q, want %q", outPath, expectedPath)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var rounds []ExportRound
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var er ExportRound
		if err := json.Unmarshal(scanner.Bytes(), &er); err != nil {
			t.Fatal(err)
		}
		rounds = append(rounds, er)
	}

	if len(rounds) != 2 {
		t.Fatalf("expected 2 export rounds, got %d", len(rounds))
	}

	r0 := rounds[0]
	if r0.SessionID != "test-session" {
		t.Errorf("session_id = %q", r0.SessionID)
	}
	if r0.UserMessage != "hello" {
		t.Errorf("user_message = %q", r0.UserMessage)
	}
	if r0.AssistantResponse != "Hi there!\nHow can I help?" {
		t.Errorf("assistant_response = %q", r0.AssistantResponse)
	}
	if len(r0.ToolCalls) != 1 || r0.ToolCalls[0].Name != "Read" {
		t.Errorf("tool_calls = %v", r0.ToolCalls)
	}
	if r0.Usage.InputTokens != 100 || r0.Usage.OutputTokens != 50 {
		t.Errorf("usage = %+v", r0.Usage)
	}
	if len(r0.ThinkingTexts) != 1 || r0.ThinkingTexts[0] != "let me think about this" {
		t.Errorf("thinking_texts = %v", r0.ThinkingTexts)
	}

	// Round 1 should have no thinking.
	r1 := rounds[1]
	if len(r1.ThinkingTexts) != 0 {
		t.Errorf("round 1 thinking_texts should be empty, got %v", r1.ThinkingTexts)
	}
}

func TestExportSessionMarkdown(t *testing.T) {
	configDir := t.TempDir()

	session := SessionSummary{
		SessionID:   "md-session",
		Project:     "/Users/kfu/code/bar",
		ProjectName: "bar",
	}

	transcript := &Transcript{
		SessionID: "md-session",
		Rounds: []Round{
			{
				Index:          0,
				UserMessage:    "explain this code",
				UserTimestamp:   "2026-02-26T19:02:00Z",
				AssistantTexts: []string{"This code does XYZ."},
				ThinkingTexts:  []string{"chain of thought here"},
				ToolCalls: []ToolCall{
					{Name: "Read", InputSummary: "/path/to/file.go"},
					{Name: "Bash", InputSummary: "ls -la"},
				},
				Usage: Usage{InputTokens: 3, OutputTokens: 720, CacheRead: 34582, CacheCreation: 1593},
			},
		},
	}

	outPath, err := ExportSessionMarkdown(configDir, session, transcript, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	expectedPath := filepath.Join(configDir, "exports", "md-session.md")
	if outPath != expectedPath {
		t.Errorf("output path = %q, want %q", outPath, expectedPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"# Session md-session",
		"**Project**: /Users/kfu/code/bar",
		"**Rounds**: 1",
		"## Round 1 (2026-02-26T19:02:00Z)",
		"```prompt\nexplain this code\n```",
		"```tool_use\nRead: /path/to/file.go\nBash: ls -la\n```",
		"```thinking\nchain of thought here\n```",
		"```assistant\nThis code does XYZ.\n```",
		"> Tokens: in=3 out=720 cache_read=34582 cache_write=1593",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("markdown missing: %q", check)
		}
	}
}

func TestExportSessionMarkdownNoThinking(t *testing.T) {
	configDir := t.TempDir()

	session := SessionSummary{SessionID: "no-think", Project: "/foo"}
	transcript := &Transcript{
		SessionID: "no-think",
		Rounds: []Round{
			{
				Index:          0,
				UserMessage:    "hi",
				UserTimestamp:   "2026-02-26T19:00:00Z",
				AssistantTexts: []string{"hello"},
				ThinkingTexts:  []string{"secret thoughts"},
				Usage:          Usage{OutputTokens: 10},
			},
		},
	}

	outPath, err := ExportSessionMarkdown(configDir, session, transcript, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "thinking") {
		t.Error("markdown should not contain thinking when includeThinking=false")
	}
}

func TestExportSessionRoundsSubset(t *testing.T) {
	configDir := t.TempDir()

	session := SessionSummary{SessionID: "sub", Project: "/foo"}
	transcript := &Transcript{
		SessionID: "sub",
		Rounds: []Round{
			{Index: 0, UserMessage: "r0", UserTimestamp: "t0", AssistantTexts: []string{"a0"}},
			{Index: 1, UserMessage: "r1", UserTimestamp: "t1", AssistantTexts: []string{"a1"}},
			{Index: 2, UserMessage: "r2", UserTimestamp: "t2", AssistantTexts: []string{"a2"}},
		},
	}

	outPath, err := ExportSessionRounds(configDir, session, transcript, []int{0, 2}, true)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var rounds []ExportRound
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var er ExportRound
		if err := json.Unmarshal(scanner.Bytes(), &er); err != nil {
			t.Fatal(err)
		}
		rounds = append(rounds, er)
	}

	if len(rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(rounds))
	}
	if rounds[0].RoundIndex != 0 || rounds[1].RoundIndex != 2 {
		t.Errorf("round indices = %d, %d", rounds[0].RoundIndex, rounds[1].RoundIndex)
	}
}

func TestConfigDir(t *testing.T) {
	// With XDG set.
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got := ConfigDir()
	if got != "/tmp/xdg/cc-viewer" {
		t.Errorf("got %q with XDG set", got)
	}

	// Without XDG.
	t.Setenv("XDG_CONFIG_HOME", "")
	got = ConfigDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "cc-viewer")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
