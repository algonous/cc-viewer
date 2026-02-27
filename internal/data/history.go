package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// historyEntry is the raw JSON structure of one line in history.jsonl.
type historyEntry struct {
	SessionID      string                    `json:"sessionId"`
	Timestamp      int64                     `json:"timestamp"`
	Project        string                    `json:"project"`
	Display        string                    `json:"display"`
	PastedContents map[string]pastedContent  `json:"pastedContents,omitempty"`
}

// pastedContent is the structure of a pasted text block in history.jsonl.
type pastedContent struct {
	Content string `json:"content"`
}

// LoadSessions reads the history.jsonl file and discovers orphaned transcript
// files (sessions that fell off the history.jsonl 2000-line cap).
// Returns all sessions sorted by most recent first.
func LoadSessions(claudeDir string) ([]SessionSummary, error) {
	groups := make(map[string]*SessionSummary)
	messages := make(map[string][]string)
	var order []string

	// Phase 1: Parse history.jsonl for indexed sessions.
	path := filepath.Join(claudeDir, "history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var e historyEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.SessionID == "" {
			continue
		}

		s, ok := groups[e.SessionID]
		if !ok {
			s = &SessionSummary{
				SessionID:    e.SessionID,
				Project:      e.Project,
				ProjectName:  projectName(e.Project),
				FirstMessage: e.Display,
				FirstTS:      e.Timestamp,
				LastTS:       e.Timestamp,
			}
			groups[e.SessionID] = s
			order = append(order, e.SessionID)
		}
		s.MessageCount++
		if e.Display != "" && e.Display != "exit" {
			messages[e.SessionID] = append(messages[e.SessionID], e.Display)
		}
		for _, pc := range e.PastedContents {
			if pc.Content != "" {
				messages[e.SessionID] = append(messages[e.SessionID], pc.Content)
			}
		}
		if e.Timestamp < s.FirstTS {
			s.FirstTS = e.Timestamp
			s.FirstMessage = e.Display
		}
		if e.Timestamp > s.LastTS {
			s.LastTS = e.Timestamp
		}
	}
	f.Close()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Phase 2: Scan transcript files for orphaned sessions not in history.jsonl.
	discoverOrphanSessions(claudeDir, groups, messages, &order)

	result := make([]SessionSummary, 0, len(groups))
	for _, id := range order {
		s := groups[id]
		s.AllMessages = strings.Join(messages[id], "\n")
		result = append(result, *s)
	}

	// Sort by most recent first.
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTS > result[j].LastTS
	})

	return result, nil
}

// discoverOrphanSessions finds transcript JSONL files that have no entry in
// history.jsonl (due to the 2000-line cap) and adds them to the session index.
// It reads the first user message from each orphan transcript for search.
func discoverOrphanSessions(claudeDir string, groups map[string]*SessionSummary, messages map[string][]string, order *[]string) {
	projectsDir := filepath.Join(claudeDir, "projects")
	projEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	for _, projEntry := range projEntries {
		if !projEntry.IsDir() {
			continue
		}
		projPath := filepath.Join(projectsDir, projEntry.Name())
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}
		for _, file := range files {
			name := file.Name()
			if file.IsDir() || !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			// Skip subagent files (agent-* prefix).
			if strings.HasPrefix(name, "agent-") {
				continue
			}
			sid := strings.TrimSuffix(name, ".jsonl")
			if _, ok := groups[sid]; ok {
				continue // already in history.jsonl
			}

			// Orphan session -- extract metadata from transcript.
			s := orphanSessionFromTranscript(filepath.Join(projPath, name), sid, projEntry.Name())
			if s == nil {
				continue
			}
			groups[sid] = s
			messages[sid] = []string{s.FirstMessage}
			*order = append(*order, sid)
		}
	}
}

// orphanSessionFromTranscript reads the first few lines of a transcript to
// extract basic session metadata for orphaned sessions.
func orphanSessionFromTranscript(path string, sid string, encodedProject string) *SessionSummary {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	info, _ := f.Stat()

	// Decode project path from directory name (- -> /).
	project := strings.ReplaceAll(encodedProject, "-", "/")

	s := &SessionSummary{
		SessionID:   sid,
		Project:     project,
		ProjectName: projectName(project),
	}

	if info != nil {
		s.LastTS = info.ModTime().UnixMilli()
	}

	// Scan for first user message.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry transcriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type != "user" || entry.Message == nil {
			if entry.Timestamp != "" && s.FirstTS == 0 {
				// Use first entry timestamp as approximate start time.
				// Parse ISO 8601 timestamp.
				if t, err := parseTimestamp(entry.Timestamp); err == nil {
					s.FirstTS = t
				}
			}
			continue
		}
		var msg transcriptMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}
		content := strings.TrimSpace(string(msg.Content))
		if len(content) > 0 && content[0] == '"' {
			var text string
			if err := json.Unmarshal(msg.Content, &text); err == nil && text != "" {
				s.FirstMessage = text
				if s.FirstTS == 0 {
					if t, err := parseTimestamp(entry.Timestamp); err == nil {
						s.FirstTS = t
					}
				}
				s.MessageCount = 1
				return s
			}
		}
	}

	// No user message found -- still return with file-based metadata.
	if s.FirstTS == 0 {
		s.FirstTS = s.LastTS
	}
	return s
}

// parseTimestamp parses an ISO 8601 timestamp string to unix milliseconds.
func parseTimestamp(ts string) (int64, error) {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", ts)
		if err != nil {
			return 0, err
		}
	}
	return t.UnixMilli(), nil
}

// projectName extracts the last path component from an absolute project path.
func projectName(project string) string {
	if project == "" {
		return ""
	}
	return filepath.Base(strings.TrimRight(project, "/"))
}

// FindTranscriptPath locates the transcript JSONL file for a given session.
// It searches through all project directories under claudeDir/projects/.
func FindTranscriptPath(claudeDir string, sessionID string) (string, error) {
	projectsDir := filepath.Join(claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", err
	}

	filename := sessionID + ".jsonl"
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(projectsDir, entry.Name(), filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", os.ErrNotExist
}

// EncodeProjectDir encodes an absolute path to the directory name format
// used under ~/.claude/projects/ (replace "/" with "-").
func EncodeProjectDir(absPath string) string {
	return strings.ReplaceAll(absPath, "/", "-")
}
