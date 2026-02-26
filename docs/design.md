# Design

## Data Model

### Session Index (history.jsonl)

- Path: `~/.claude/history.jsonl`
- Format: one JSON line per user message
- Fields: `sessionId`, `timestamp` (unix ms), `project` (abs path), `display`
- Grouped by sessionId -> SessionSummary
- Sessions with `"display": "exit"` entry are closed; those without may still be alive

### Transcript (per-session JSONL)

- Path: `~/.claude/projects/<encoded-project>/<sessionId>.jsonl`
- Encoding: `/` -> `-` (e.g. `/Users/kfu/code/foo` -> `-Users-kfu-code-foo`)
- Entry types: `user`, `assistant`, `progress`, `file-history-snapshot`
- Round segmentation: user message with string content starts a new round;
  tool_result user entries continue the current round

### Content Blocks (within assistant entries)

```json
{"type": "text", "text": "..."}
{"type": "thinking", "thinking": "..."}
{"type": "tool_use", "name": "Read", "input": {...}}
```

### Usage Tracking

Per assistant entry:
```json
{
  "input_tokens": 3,
  "output_tokens": 720,
  "cache_creation_input_tokens": 1593,
  "cache_read_input_tokens": 34582
}
```

## Web Architecture

### Server (Go)

- Single binary with `go:embed web/` for static assets
- Go 1.22+ ServeMux with path patterns
- SPA fallback: all non-API GET requests serve index.html
- md2html renders markdown content within fields (not block structure)

### API

```
GET  /api/sessions              -> [{session_id, project, project_name, ...}]
GET  /api/transcript/{id}       -> {session_id, rounds: [{index, user_html, assistant_html, thinking_html, tool_calls, usage}]}
POST /api/export                -> binary download (Content-Disposition: attachment)
```

The transcript endpoint sends structured JSON per round. HTML fields contain rendered markdown. Tool calls are structured data (name + input_summary), not HTML.

### Frontend (vanilla JS)

Mouse-only interaction. No keyboard shortcuts (browser extensions like Vimium conflict with single-letter keys).

State:
- sessions, filteredSessions, currentSession, transcript
- selectedBlocks (checkbox state per block)
- exportFormat (jsonl or md)

Block types with fold behavior:
- YOU, CLAUDE: open by default
- CONTEXT, TOOL, THINKING: folded by default

Each block has: checkbox, fold arrow, role label, fold summary (shown when collapsed), fold body (shown when expanded).

## Export Formats

### JSONL

One line per round:
```json
{
  "session_id": "...",
  "timestamp": "...",
  "project": "...",
  "round_index": 0,
  "user_message": "...",
  "tool_calls": [{"name": "Read", "input_summary": "..."}],
  "assistant_response": "...",
  "thinking_texts": ["..."],
  "usage": {"input_tokens": 3, "output_tokens": 720, "cache_read": 34582, "cache_creation": 1593}
}
```

### Markdown

```
# Session <id>

- **Project**: /path/to/project
- **Rounds**: 10
- **Total tokens**: in=1000 out=500 cache_read=8000 cache_write=200

---

## Round 1 (2026-02-27T10:00:00Z)

` ``prompt
user message
` ``

` ``tool_use
Read: /path/to/file.go
Bash: ls -la
` ``

` ``thinking
chain of thought
` ``

` ``assistant
response text
` ``

> Tokens: in=3 out=720 cache_read=34582 cache_write=1593
```

## Theme

Color palette matches [md2html](https://github.com/algonous/md2html):

| Block    | Border    | Background | Role text |
|----------|-----------|------------|-----------|
| YOU      | `#d97706` | `#fefce8`  | `#854d0e` |
| CLAUDE   | `#3b82f6` | `#eff6ff`  | `#1e40af` |
| TOOL     | `#22c55e` | `#f0fdf4`  | `#166534` |
| CONTEXT  | `#a78bfa` | `#f5f3ff`  | `#5b21b6` |
| THINKING | `#9ca3af` | `#f9fafb`  | `#4b5563` |

Page background: `#f5f5f3`. Code blocks: `#f6f8fa` bg, `#e1e4e8` border.
