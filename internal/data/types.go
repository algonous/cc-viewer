package data

// SessionSummary is derived from history.jsonl by grouping entries by SessionID.
type SessionSummary struct {
	SessionID    string
	Project      string
	ProjectName  string
	FirstMessage string
	FirstTS      int64 // unix ms
	LastTS       int64 // unix ms
	MessageCount int
}

// Round represents one user turn and the assistant response(s) that follow.
type Round struct {
	Index          int
	UserMessage    string
	UserTimestamp  string
	IsContext      bool // true if UserMessage is system-injected context, not user input
	AssistantTexts []string
	ThinkingTexts  []string
	ToolCalls      []ToolCall
	Usage          Usage // aggregated across all assistant entries in this round
}

// ToolCall represents a single tool invocation within a round.
type ToolCall struct {
	Name         string
	InputSummary string
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

// ExportRound is the JSON structure written per line in export files.
type ExportRound struct {
	SessionID         string       `json:"session_id"`
	Timestamp         string       `json:"timestamp"`
	Project           string       `json:"project"`
	RoundIndex        int          `json:"round_index"`
	IsContext         bool         `json:"is_context"`
	UserMessage       string       `json:"user_message"`
	ToolCalls         []ExportTool `json:"tool_calls"`
	AssistantResponse string       `json:"assistant_response"`
	ThinkingTexts     []string     `json:"thinking_texts,omitempty"`
	Usage             ExportUsage  `json:"usage"`
}

// ExportTool is the tool call representation in export files.
type ExportTool struct {
	Name         string `json:"name"`
	InputSummary string `json:"input_summary"`
}

// ExportUsage is the usage representation in export files.
type ExportUsage struct {
	InputTokens   int64 `json:"input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	CacheRead     int64 `json:"cache_read"`
	CacheCreation int64 `json:"cache_creation"`
}
