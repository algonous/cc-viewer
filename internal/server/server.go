package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/algonous/cc-viewer/internal/data"
	"github.com/algonous/md2html/md2html"
)

// Server handles HTTP requests for the cc-viewer web UI.
type Server struct {
	claudeDir string
	sessions  []data.SessionSummary
	webFS     fs.FS
}

// New creates a new Server.
func New(claudeDir string, sessions []data.SessionSummary, webFS fs.FS) *Server {
	return &Server{claudeDir: claudeDir, sessions: sessions, webFS: webFS}
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
	FirstTS      int64  `json:"first_ts"`
	LastTS       int64  `json:"last_ts"`
	MessageCount int    `json:"message_count"`
}

type transcriptJSON struct {
	SessionID string      `json:"session_id"`
	Rounds    []roundJSON `json:"rounds"`
}

// roundJSON sends structured data per round.  The frontend owns layout.
type roundJSON struct {
	Index         int            `json:"index"`
	UserTimestamp string         `json:"user_timestamp"`
	IsContext     bool           `json:"is_context"`
	UserHTML      string         `json:"user_html,omitempty"`
	AssistantHTML string         `json:"assistant_html,omitempty"`
	ThinkingHTML  string         `json:"thinking_html,omitempty"`
	ToolCalls     []toolCallJSON `json:"tool_calls,omitempty"`
	Usage         usageJSON      `json:"usage"`
}

type toolCallJSON struct {
	Name         string `json:"name"`
	InputSummary string `json:"input_summary,omitempty"`
}

type usageJSON struct {
	InputTokens   int64 `json:"input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	CacheRead     int64 `json:"cache_read"`
	CacheCreation int64 `json:"cache_creation"`
}

type exportRequest struct {
	SessionID       string `json:"session_id"`
	Format          string `json:"format"`
	RoundIndices    []int  `json:"round_indices,omitempty"`
	IncludeThinking bool   `json:"include_thinking"`
}

// --- Handlers ---

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	result := make([]sessionJSON, len(s.sessions))
	for i, sess := range s.sessions {
		result[i] = sessionJSON{
			SessionID:    sess.SessionID,
			Project:      sess.Project,
			ProjectName:  sess.ProjectName,
			FirstMessage: sess.FirstMessage,
			FirstTS:      sess.FirstTS,
			LastTS:       sess.LastTS,
			MessageCount: sess.MessageCount,
		}
	}
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
		// Tool calls as structured data.
		var tools []toolCallJSON
		if len(rd.ToolCalls) > 0 {
			tools = make([]toolCallJSON, len(rd.ToolCalls))
			for j, tc := range rd.ToolCalls {
				tools[j] = toolCallJSON{Name: tc.Name, InputSummary: tc.InputSummary}
			}
		}

		rounds[i] = roundJSON{
			Index:         rd.Index,
			UserTimestamp:  rd.UserTimestamp,
			IsContext:      rd.IsContext,
			UserHTML:       renderMarkdown(rd.UserMessage),
			AssistantHTML:  renderMarkdown(strings.Join(rd.AssistantTexts, "\n")),
			ThinkingHTML:   renderMarkdown(strings.Join(rd.ThinkingTexts, "\n\n")),
			ToolCalls:      tools,
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
	var session data.SessionSummary
	found := false
	for _, sess := range s.sessions {
		if sess.SessionID == req.SessionID {
			session = sess
			found = true
			break
		}
	}
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

	var indices []int
	if len(req.RoundIndices) > 0 {
		indices = req.RoundIndices
	}

	var content []byte
	var filename string
	var contentType string
	switch req.Format {
	case "md":
		content = data.GenerateMarkdown(session, transcript, indices, req.IncludeThinking)
		filename = req.SessionID + ".md"
		contentType = "text/markdown; charset=utf-8"
	default:
		content = data.GenerateJSONL(session, transcript, indices, req.IncludeThinking)
		filename = req.SessionID + ".jsonl"
		contentType = "application/x-ndjson"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(content)
}

// --- Rendering ---

// renderMarkdown converts a markdown string to HTML using the md2html pipeline.
// Used to render content *within* a block, not the block structure itself.
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
