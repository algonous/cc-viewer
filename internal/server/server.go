package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/algonous/cc-viewer/internal/data"
	"github.com/algonous/md2html/md2html"
)

// Server handles HTTP requests for the cc-viewer web UI.
type Server struct {
	claudeDir   string
	sessions    []data.SessionSummary
	webFS       fs.FS
	mu          sync.Mutex
	lastRefresh time.Time
}

// New creates a new Server.
func New(claudeDir string, sessions []data.SessionSummary, webFS fs.FS) *Server {
	return &Server{claudeDir: claudeDir, sessions: sessions, webFS: webFS}
}

// refreshSessions re-reads history.jsonl and merges new or updated sessions.
func (s *Server) refreshSessions() {
	fresh, err := data.LoadHistoryQuick(s.claudeDir)
	if err != nil {
		return
	}

	existing := make(map[string]int, len(s.sessions))
	for i, sess := range s.sessions {
		existing[sess.SessionID] = i
	}

	changed := false
	for _, sess := range fresh {
		if idx, ok := existing[sess.SessionID]; ok {
			if sess.LastTS > s.sessions[idx].LastTS {
				s.sessions[idx].LastTS = sess.LastTS
				changed = true
			}
		} else {
			s.sessions = append(s.sessions, sess)
			changed = true
		}
	}

	if changed {
		sort.Slice(s.sessions, func(i, j int) bool {
			return s.sessions[i].LastTS > s.sessions[j].LastTS
		})
	}
}

// Handler returns the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Embedded static files.
	staticFS, _ := fs.Sub(s.webFS, "web/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// SPA fallback: serve index.html for all non-API, non-static GET requests.
	indexHTML, _ := fs.ReadFile(s.webFS, "web/index.html")
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	// API endpoints.
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/transcript/{id}", s.handleTranscript)
	mux.HandleFunc("POST /api/export", s.handleExport)

	return mux
}

// --- JSON response types ---

type sessionJSON struct {
	SessionID    string `json:"session_id"`
	Project      string `json:"project"`
	ProjectName  string `json:"project_name"`
	FirstMessage string `json:"first_message"`
	AllMessages  string `json:"all_messages"`
	FirstTS      int64  `json:"first_ts"`
	LastTS       int64  `json:"last_ts"`
	MessageCount int    `json:"message_count"`
}

type transcriptJSON struct {
	SessionID string      `json:"session_id"`
	Rounds    []roundJSON `json:"rounds"`
}

type roundJSON struct {
	Index         int         `json:"index"`
	UserTimestamp string      `json:"user_timestamp"`
	IsContext     bool        `json:"is_context"`
	Blocks        []blockJSON `json:"blocks"`
	Usage         usageJSON   `json:"usage"`
}

type blockJSON struct {
	Role         string `json:"role"`
	HTML         string `json:"html,omitempty"`
	Name         string `json:"name,omitempty"`
	InputSummary string `json:"input_summary,omitempty"`
}

type usageJSON struct {
	InputTokens   int64 `json:"input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	CacheRead     int64 `json:"cache_read"`
	CacheCreation int64 `json:"cache_creation"`
}

type exportRequest struct {
	SessionID string  `json:"session_id"`
	Format    string  `json:"format"`
	Blocks    [][]int `json:"blocks,omitempty"`
}

// --- Handlers ---

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if time.Since(s.lastRefresh) > 5*time.Second {
		s.refreshSessions()
		s.lastRefresh = time.Now()
	}
	result := make([]sessionJSON, len(s.sessions))
	for i, sess := range s.sessions {
		result[i] = sessionJSON{
			SessionID:    sess.SessionID,
			Project:      sess.Project,
			ProjectName:  sess.ProjectName,
			FirstMessage: sess.FirstMessage,
			AllMessages:  sess.AllMessages,
			FirstTS:      sess.FirstTS,
			LastTS:       sess.LastTS,
			MessageCount: sess.MessageCount,
		}
	}
	s.mu.Unlock()
	writeJSON(w, result)
}

func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	path, err := data.FindTranscriptPath(s.claudeDir, id)
	if err != nil {
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}

	transcript, err := data.LoadTranscript(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rounds := make([]roundJSON, len(transcript.Rounds))
	for i, rd := range transcript.Rounds {
		blocks := make([]blockJSON, len(rd.Blocks))
		for j, b := range rd.Blocks {
			if b.Role == "tool" && b.ToolCall != nil {
				blocks[j] = blockJSON{
					Role:         "tool",
					Name:         b.ToolCall.Name,
					InputSummary: b.ToolCall.InputSummary,
				}
			} else {
				blocks[j] = blockJSON{
					Role: b.Role,
					HTML: renderMarkdown(b.Text),
				}
			}
		}

		rounds[i] = roundJSON{
			Index:         rd.Index,
			UserTimestamp:  rd.UserTimestamp,
			IsContext:      rd.IsContext,
			Blocks:        blocks,
			Usage: usageJSON{
				InputTokens:   rd.Usage.InputTokens,
				OutputTokens:  rd.Usage.OutputTokens,
				CacheRead:     rd.Usage.CacheRead,
				CacheCreation: rd.Usage.CacheCreation,
			},
		}
	}

	writeJSON(w, transcriptJSON{SessionID: id, Rounds: rounds})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Find session.
	s.mu.Lock()
	var session data.SessionSummary
	found := false
	for _, sess := range s.sessions {
		if sess.SessionID == req.SessionID {
			session = sess
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Load transcript.
	path, err := data.FindTranscriptPath(s.claudeDir, req.SessionID)
	if err != nil {
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}
	transcript, err := data.LoadTranscript(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert request blocks from [][]int to [][2]int.
	var blocks [][2]int
	if len(req.Blocks) > 0 {
		blocks = make([][2]int, 0, len(req.Blocks))
		for _, pair := range req.Blocks {
			if len(pair) == 2 {
				blocks = append(blocks, [2]int{pair[0], pair[1]})
			}
		}
	}

	var content []byte
	var filename string
	var contentType string
	switch req.Format {
	case "md":
		content = data.GenerateMarkdown(session, transcript, blocks)
		filename = req.SessionID + ".md"
		contentType = "text/markdown; charset=utf-8"
	default:
		content = data.GenerateJSONL(session, transcript, blocks)
		filename = req.SessionID + ".jsonl"
		contentType = "application/x-ndjson"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(content)
}

// --- Rendering ---

// renderMarkdown converts a markdown string to HTML using the md2html pipeline.
func renderMarkdown(text string) string {
	if text == "" {
		return ""
	}
	ast, err := md2html.ParseMarkdownToAST(text)
	if err != nil {
		return htmlEscape(text)
	}
	ir := md2html.ASTToIR(ast)
	return md2html.IRToHTML(ir)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}
