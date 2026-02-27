package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// historyEntry is the raw JSON structure of one line in history.jsonl.
type historyEntry struct {
	SessionID string `json:"sessionId"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
	Display   string `json:"display"`
}

// LoadSessions reads the history.jsonl file and returns sessions grouped by ID,
// sorted by most recent first.
func LoadSessions(claudeDir string) ([]SessionSummary, error) {
	path := filepath.Join(claudeDir, "history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	groups := make(map[string]*SessionSummary)
	messages := make(map[string][]string)
	var order []string

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
		if e.Timestamp < s.FirstTS {
			s.FirstTS = e.Timestamp
			s.FirstMessage = e.Display
		}
		if e.Timestamp > s.LastTS {
			s.LastTS = e.Timestamp
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

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
