package server

import (
	"strings"
	"testing"

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
