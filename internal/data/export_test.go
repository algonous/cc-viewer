package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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
}

func TestConfigDir(t *testing.T) {
	// With XDG set.
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got := ConfigDir()
	if got != "/tmp/xdg/cc-tree" {
		t.Errorf("got %q with XDG set", got)
	}

	// Without XDG.
	t.Setenv("XDG_CONFIG_HOME", "")
	got = ConfigDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "cc-tree")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
