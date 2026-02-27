# cc-viewer: Claude Code Session Browser

## Overview

A web-based application that browses Claude Code conversation history stored locally under `~/.claude/`.

## Features

- Browse all Claude Code sessions with timestamps and project info
- Distinguish live (still running) vs exited sessions
- View full transcripts organized by rounds; rounds ordered by time, blocks within each round ordered by time
- Fold/unfold blocks (context, tool, thinking folded by default)
- Token usage tracking per round (input, output, cache read, cache creation)
- Select blocks with checkboxes, copy text to clipboard
- Export sessions to JSONL or Markdown (downloads to browser)
- Filter sessions by project or content
- URL routing: `/<sessionId>` links directly to a session

## Data Format (~/.claude/)

### Directory layout

```
~/.claude/
  history.jsonl                              # session index (one line per user message)
  projects/
    -Users-frank-code-foo/                   # encoded project path (/ -> -)
      <sessionId>.jsonl                      # transcript for one session
      agent-<id>.jsonl                       # subagent transcripts (not used)
    -Users-frank-code-bar/
      ...
```

### history.jsonl -- session index

One JSON line per user message. Fields:

| Field            | Type              | Notes                                            |
|------------------|-------------------|--------------------------------------------------|
| `sessionId`      | string            | Groups lines into sessions                       |
| `timestamp`      | int64 (unix ms)   | When the message was sent                        |
| `project`        | string            | Absolute path of the working directory           |
| `display`        | string            | User message text, or `"exit"` for session close |
| `pastedContents` | map[string]object | Optional; key is a numeric string ID (e.g. `"1"`), value has `id` (int), `type` (string), `content` (string) |

Grouping logic: all lines sharing the same `sessionId` form one session. A session with a `"display": "exit"` entry is closed; those without may still be running.

Resuming a session (`claude --resume`) has two behaviors depending on whether the context window overflowed:

- **Context intact**: same session ID, same transcript file. Claude Code appends new entries seamlessly -- `"exit"` does not permanently seal a session.
- **Context overflowed**: new session ID, new transcript file. The first user message is a "continued from a previous conversation" summary injected by Claude Code (see "System-injected context" below).

All session metadata (timestamps, project, user messages, pasted content) is also available in the transcript files. However, history.jsonl is the only source of `"exit"` markers, which is needed to distinguish live sessions from exited ones.

Claude Code caps history.jsonl at ~2000 lines. Each line is either a user-initiated round or an `"exit"` marker, so the cap is ~2000 rounds across all sessions. Older sessions fall off this file but their transcript JSONL files remain on disk indefinitely (Claude Code never cleans them up).

### Transcript files -- per-session JSONL

Path: `~/.claude/projects/<encoded-project>/<sessionId>.jsonl`

Path encoding: every `/` in the absolute project path becomes `-`.
Example: `/Users/frank/code/foo` -> `-Users-frank-code-foo`

Each line is a JSON object with a `type` field. Relevant types:

| type                     | Description                                  |
|--------------------------|----------------------------------------------|
| `user`                   | User message or tool_result                  |
| `assistant`              | One model response (contains content blocks) |
| `progress`               | Streaming progress                           |
| `file-history-snapshot`  | File state snapshot                          |

`tool_use` and `thinking` are not top-level entry types. They are content block types *within* `assistant` entries (see "Content block types" below).

**Round segmentation**:

- A `user` entry whose `message.content` is a **JSON string** (a real user message) starts a **new round**.
- A `user` entry whose `message.content` is a **JSON array** (tool_result) continues the current round.
- All `assistant` entries following a user message belong to the same round. Thinking and tool_use blocks are inside these assistant entries and do not affect round boundaries.

**System-injected context**: Claude Code writes special user messages in specific situations. These are regular `user` entries but carry system context rather than real user input:

- `"This session is being continued from a previous conversation..."` -- written by `claude --resume` when a session ran out of context and is being continued with a summary of the prior conversation.
- `"Implement the following plan:"` -- written when Claude Code exits plan mode and begins implementing the approved plan.

### Content block types

Each `assistant` entry has `message.content` as a JSON array of content blocks. Each entry typically contains one block, but multiple assistant entries chain within the same round.

```json
{"type": "text", "text": "Here is the implementation..."}
```

```json
{"type": "thinking", "thinking": "Let me consider the approach..."}
```

```json
{"type": "tool_use", "name": "Read", "input": {"file_path": "/foo/bar.go"}}
```

### Usage tracking

Each assistant entry carries a `message.usage` object:

```json
{
  "input_tokens": 3,
  "output_tokens": 720,
  "cache_creation_input_tokens": 1593,
  "cache_read_input_tokens": 34582
}
```

Multiple assistant entries in the same round share the same `input_tokens` and `cache_*` values (they repeat per entry in the same API call), while `output_tokens` accumulates across entries.
