package data

// SessionSummary is derived from history.jsonl by grouping entries by SessionID.
type SessionSummary struct {
	SessionID    string
	Project      string
	ProjectName  string
	FilePath     string // absolute path to the session JSONL file
	FirstMessage string
	AllMessages  string // all user messages concatenated, for search
	FirstTS      int64  // unix ms
	LastTS       int64  // unix ms
	MessageCount int
}

// Block represents a single visible block within a round.
// Blocks are stored in file order (chronological).
type Block struct {
	Role     string    // "you", "context", "tool", "thinking", "claude"
	Text     string    // raw text content (empty for tool blocks)
	ToolCall *ToolCall // non-nil for tool blocks only
}

// Round represents one user turn and the assistant response(s) that follow.
type Round struct {
	Index         int
	UserTimestamp string
	IsContext     bool // true if user message is system-injected context
	Blocks        []Block
	Usage         Usage // aggregated across all assistant entries in this round
}

// ToolCall represents a single tool invocation within a round.
type ToolCall struct {
	Name         string
	InputSummary string
	InputJSON    string // prettified JSON of the full input, for display
}

// Usage holds token counts aggregated from assistant message.usage fields.
type Usage struct {
	InputTokens   int64
	OutputTokens  int64
	CacheCreation int64
	CacheRead     int64
}

// Transcript is the parsed content of a session transcript file.
type Transcript struct {
	SessionID string
	Rounds    []Round
}

// ExportBlock is the JSON structure written per line in JSONL export files.
type ExportBlock struct {
	SessionID    string `json:"session_id"`
	RoundIndex   int    `json:"round_index"`
	BlockIndex   int    `json:"block_index"`
	Role         string `json:"role"`
	Text         string `json:"text,omitempty"`
	Name         string `json:"name,omitempty"`
	InputSummary string `json:"input_summary,omitempty"`
}
