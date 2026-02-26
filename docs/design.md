# Design

## Data Model

### Session Index (history.jsonl)

- Path: `~/.claude/history.jsonl`
- Format: one JSON line per user message
- Fields: `sessionId`, `timestamp` (unix ms), `project` (abs path), `display`
- Grouped by sessionId -> SessionSummary

### Transcript (per-session JSONL)

- Path: `~/.claude/projects/<encoded-project>/<sessionId>.jsonl`
- Encoding: `/` -> `-` (e.g. `/Users/kfu/code/foo` -> `-Users-kfu-code-foo`)
- Entry types: `user`, `assistant`, `progress`, `file-history-snapshot`
- Round segmentation: user message with string content starts a new round;
  tool_result user entries continue the current round

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

## TUI Architecture

Split-pane layout using Bubble Tea:
- Left: session list (bubbles/list) -- 40 chars wide
- Right: transcript viewer (bubbles/viewport) -- remaining width
- Bottom: status bar with key hints

## Export Format

One JSONL line per round:
```json
{
  "session_id": "...",
  "timestamp": "...",
  "project": "...",
  "round_index": 0,
  "user_message": "...",
  "tool_calls": [{"name": "Read", "input_summary": "...", "output_summary": "..."}],
  "assistant_response": "...",
  "usage": {"input_tokens": 3, "output_tokens": 720, "cache_read": 34582, "cache_creation": 1593}
}
```
