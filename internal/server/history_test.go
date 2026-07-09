package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/algonous/agent-sessions/internal/data"
)

func TestProcessHistoryLineMaintainsSearchText(t *testing.T) {
	srv := New(nil, []data.SessionSummary{{
		SessionID:    data.MakeSessionKey(data.SourceCodex, "s1"),
		RawSessionID: "s1",
		Source:       data.SourceCodex,
		DataDir:      "/tmp/codex",
		FirstMessage: "old message",
		AllMessages:  "old message",
		FirstTS:      1000,
		LastTS:       1000,
		MessageCount: 1,
	}}, nil)

	srv.processHistoryLine([]byte(`{"session_id":"s1","ts":2000,"text":"Glue AccessDenied latest update"}`), data.SourceCodex, "/tmp/codex")
	srv.processHistoryLine([]byte(`{"session_id":"s2","ts":3000,"text":"new Glue AccessDenied session"}`), data.SourceCodex, "/tmp/codex")

	srv.mu.RLock()
	sessions := srv.buildSessionList()
	srv.mu.RUnlock()

	existing := findSessionJSON(t, sessions, data.MakeSessionKey(data.SourceCodex, "s1"))
	if !strings.Contains(existing.AllMessages, "old message") {
		t.Fatalf("existing AllMessages lost old text: %q", existing.AllMessages)
	}
	if !strings.Contains(existing.AllMessages, "glue accessdenied latest update") {
		t.Fatalf("existing AllMessages missing live text: %q", existing.AllMessages)
	}
	if existing.MessageCount != 2 {
		t.Fatalf("existing MessageCount = %d, want 2", existing.MessageCount)
	}
	if existing.LastTS != 2000 {
		t.Fatalf("existing LastTS = %d, want 2000", existing.LastTS)
	}

	created := findSessionJSON(t, sessions, data.MakeSessionKey(data.SourceCodex, "s2"))
	if created.FirstMessage != "new Glue AccessDenied session" {
		t.Fatalf("created FirstMessage = %q", created.FirstMessage)
	}
	if created.AllMessages != "new glue accessdenied session" {
		t.Fatalf("created AllMessages = %q", created.AllMessages)
	}
}

func TestProcessHistoryLineIndexesClaudePastedContents(t *testing.T) {
	srv := New(nil, nil, nil)
	line := []byte(`{"sessionId":"c1","timestamp":1000,"project":"/tmp/proj","display":"please **inspect**","pastedContents":{"a":{"content":"pasted Glue AccessDenied log"}}}`)

	srv.processHistoryLine(line, data.SourceClaude, "/tmp/claude")

	srv.mu.RLock()
	sessions := srv.buildSessionList()
	srv.mu.RUnlock()

	created := findSessionJSON(t, sessions, data.MakeSessionKey(data.SourceClaude, "c1"))
	if !strings.Contains(created.AllMessages, "please inspect") {
		t.Fatalf("created AllMessages missing display: %q", created.AllMessages)
	}
	if !strings.Contains(created.AllMessages, "pasted glue accessdenied log") {
		t.Fatalf("created AllMessages missing pasted content: %q", created.AllMessages)
	}
}

func TestStartHistoryTailIndexesLiveCodexTranscript(t *testing.T) {
	dir := t.TempDir()
	rawID := "11111111-2222-3333-4444-555555555555"
	history := `{"session_id":"` + rawID + `","ts":1783490710,"text":"history user text"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte(history), 0644); err != nil {
		t.Fatal(err)
	}

	sessionDir := filepath.Join(dir, "sessions", "2026", "07", "08")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(sessionDir, "rollout-2026-07-08T00-00-00-"+rawID+".jsonl")
	transcript := `{"timestamp":"2026-07-08T00:00:00Z","type":"session_meta","payload":{"cwd":"/tmp/live-codex"}}
{"timestamp":"2026-07-08T00:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"history user text"}}
`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0644); err != nil {
		t.Fatal(err)
	}

	sessions, err := data.LoadSessionsMulti([]data.SourceRoot{{Source: data.SourceCodex, Dir: dir}})
	if err != nil {
		t.Fatal(err)
	}
	srv := New([]data.SourceRoot{{Source: data.SourceCodex, Dir: dir}}, sessions, nil)
	srv.StartHistoryTail()
	defer srv.StopHistoryTail()

	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintln(f, `{"timestamp":"2026-07-08T00:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"live **needle** phrase"}]}}`)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	waitForSessionText(t, srv, data.MakeSessionKey(data.SourceCodex, rawID), "live needle phrase")
}

func findSessionJSON(t *testing.T, sessions []sessionJSON, id string) sessionJSON {
	t.Helper()
	for _, sess := range sessions {
		if sess.SessionID == id {
			return sess
		}
	}
	t.Fatalf("session %s not found in %+v", id, sessions)
	return sessionJSON{}
}

func waitForSessionText(t *testing.T, srv *Server, id, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.RLock()
		sessions := srv.buildSessionList()
		srv.mu.RUnlock()
		sess := findSessionJSON(t, sessions, id)
		if strings.Contains(sess.AllMessages, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	srv.mu.RLock()
	sessions := srv.buildSessionList()
	srv.mu.RUnlock()
	sess := findSessionJSON(t, sessions, id)
	t.Fatalf("AllMessages missing %q: %q", want, sess.AllMessages)
}
