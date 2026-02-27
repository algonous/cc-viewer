# Design

See [project.md](project.md) for Claude Code's data format (`~/.claude/` directory structure, file schemas, round segmentation rules).

## Architecture

Three-layer event-driven architecture:

1. **File tail layer** -- monitors `~/.claude/` files using tail (stdlib only, no third-party dependencies). Keeps file handles open, remembers read offsets, periodically reads new data. Buffers incomplete lines (no trailing `\n`) until the next read completes them, guaranteeing each emitted event contains exactly one complete JSONL line.
2. **Event processing layer** -- receives complete JSONL lines from the tail layer. Parses JSON, maintains session state (grouping, exit status, round segmentation), renders markdown to HTML, and produces typed block events.
3. **Web layer** -- serves the frontend and pushes block events to connected clients via SSE.

Code layout:

- `internal/data/` -- data parsing layer (history index, transcript parsing, export generation)
- `internal/server/` -- HTTP handlers, md2html rendering, SSE endpoints
- `web/` -- embedded SPA (index.html, app.css, app.js)
- `main.go` -- entry point, go:embed, CLI flags

Dependencies:

- [md2html](https://github.com/algonous/md2html) -- markdown to HTML pipeline (AST -> IR -> HTML)
- Go stdlib `net/http` -- HTTP server (Go 1.22+ ServeMux)
- No frontend dependencies (vanilla CSS + JS)

## Data Flow

### File tail layer

The tail layer monitors two independent file sources. They serve different purposes and do not interact:

- **history.jsonl** -- tailed continuously from startup. Drives the sidebar (session list). New lines produce session events (new session, exit status change). Does NOT trigger any transcript file reads.
- **Transcript files** (`projects/*/<sessionId>.jsonl`) -- tailed on demand when a client clicks a session. Drives the viewer (block content). The server constructs the file path from the session's `project` field and `sessionId`. Tail stops when the client navigates away or disconnects.

Tail behavior (stdlib implementation):

1. Open file, read from offset 0 (or from last known offset for resumed tails).
2. Read with `bufio.Scanner` line by line. Each complete line (terminated by `\n`) is emitted as an event.
3. Incomplete trailing data (no `\n`) is held in the scanner's buffer. On next read attempt, new data is appended and the line completes.
4. When no new data is available, sleep briefly and retry. Files are append-only so no need to handle truncation or rewrite.

No locking is needed -- append-only files guarantee that already-read lines never change. The only race is reading a partially-written last line, which the buffering handles.

### Event processing layer

The event processing layer handles two kinds of input from the tail layer. The processing logic is the same whether the input comes from startup backlog or live tailing.

**Session events** (from history.jsonl):

Processing is the same at startup and live -- each line is handled identically:

- New `sessionId` seen -> create session summary (project, first message, timestamps).
- `"display": "exit"` -> mark session as exited. Exit status is determined by the chronologically last entry per session: exited only if the last entry is `"exit"`. If there are user messages after the last exit (session was resumed with same ID), the session is not exited.
- New user message for existing session -> update timestamps, search index.

**Block events** (from transcript files):

Processing differs between startup and client viewing because the work required is different:

- **Startup (lightweight scan)**: only extracts raw text from user and assistant entries for the full-text search index. No round/block structure, no markdown rendering. This avoids rendering markdown for all sessions when no client is looking at them.
- **Client opens a session (full rendering)**: parses round/block structure, renders markdown to HTML (using md2html), and emits block events. Each JSONL entry produces:
  - `user` with string content -> round header + YOU/CONTEXT block (starts a new round)
  - `user` with array content (tool_result) -> skip (no visible block)
  - `assistant` with content blocks -> each block individually (thinking, tool_use, or text), in file order

System-injected context messages (see project.md) are emitted as CONTEXT blocks instead of YOU blocks.

Usage aggregation per round:

- `output_tokens`: **summed** across all assistant entries (each adds new output).
- `input_tokens`, `cache_*`: **max** taken (these repeat per entry in the same API call).

**Startup sequence**:

1. **Parse history.jsonl backlog** -- process all existing lines for session summaries and exit status.
2. **Discover orphan sessions** -- scan `projects/*/` for `.jsonl` files not in history.jsonl. Without this, sessions that fell off the 2000-line cap would be invisible despite their transcripts still being on disk. Orphan sessions are treated as exited.
3. **Build full-text index** -- lightweight scan of all transcript files concurrently, extracting raw text for search.
4. **Begin tailing** -- tail layer starts monitoring history.jsonl for new session events. Transcript files are tailed on demand when a client opens a session.

### Web layer

The web layer maintains two SSE channels to the frontend:

**Session stream** (sidebar): pushes session list updates from history.jsonl tail. When a new session appears or exit status changes, the frontend updates the sidebar without a page reload.

**Transcript stream** (viewer): opened when the client clicks a session. The flow:

1. The tail layer reads the transcript file from the beginning (backlog).
2. The event processing layer renders each entry and emits block events.
3. The web layer pushes each block event to the client as an SSE message.
4. When the backlog is exhausted, the tail layer continues monitoring the file.
5. New lines (from a live session) flow through the same pipeline -- the client sees new blocks appear in real time.

Existing content and live updates use the same code path. The only difference is whether the tail layer is processing backlog or waiting for new data. When the client switches sessions, the current transcript stream is closed and a new one opens.

## Rendering

### Server-side

The transcript endpoint streams blocks as server-sent events (SSE):

1. Locates the `.jsonl` file by scanning project directories for the session ID.
2. Reads the JSONL line by line, in file order (chronological).
3. For each block, renders markdown to HTML (using md2html) and pushes it to the client. Tool calls are passed as structured data (name + input_summary), not rendered as HTML.

Each SSE event carries one block:

```json
{"round_index": 0, "role": "you", "html": "<p>rendered markdown</p>"}
```

```json
{"round_index": 0, "role": "thinking", "html": "<p>rendered markdown</p>"}
```

```json
{"round_index": 0, "role": "tool", "name": "Read", "input_summary": "/path/to/file.go"}
```

```json
{"round_index": 0, "role": "claude", "html": "<p>rendered markdown</p>"}
```

Usage is emitted as a separate event at the end of each round (or updated incrementally):

```json
{"round_index": 0, "type": "usage", "input_tokens": 3, "output_tokens": 720, "cache_read": 34582, "cache_creation": 1593}
```

### URL routing

```
/                                  -> session list (first session selected)
/<sessionId>                       -> open session, viewer starts streaming
/<sessionId>/<roundIdx>            -> open session, scroll to round
/<sessionId>/<roundIdx>/<blockIdx> -> open session, scroll to round and block
```

Round and block indices are stable -- the transcript file is append-only, so existing indices never change. New rounds/blocks get higher indices.

### Client-side

The frontend opens an SSE connection and appends each block to the DOM as it arrives. Blocks appear in chronological order within each round. If the URL includes a round or block index, the frontend scrolls to that element after it arrives.

Each block has:

- Checkbox (for selection)
- Fold arrow (triangle, rotates when open)
- Role label (uppercase)
- Fold summary (shown when collapsed: first ~80 chars of text, or tool name + input summary)
- Fold body (the full HTML content)

**Fold defaults**:

- YOU, CLAUDE: **open** by default
- CONTEXT, TOOL, THINKING: **folded** by default

**Block selection**: each block is keyed by `"b-{roundIdx}-{blockIdx}"` where `blockIdx` is assigned sequentially as blocks arrive. Selected blocks are mapped to `{roundIndex: [role, ...]}` for the export endpoint.

## Export

**API**: `POST /api/export` with body `{session_id, format, blocks}` where `blocks` is a list of `[roundIdx, blockIdx]` pairs identifying the selected blocks. When `blocks` is empty or omitted, all blocks in the session are exported.

**JSONL format**: one JSON object per selected block:

```json
{"session_id": "...", "round_index": 0, "block_index": 0, "role": "you", "text": "user message"}
{"session_id": "...", "round_index": 0, "block_index": 1, "role": "tool", "name": "Read", "input_summary": "/path/to/file.go"}
{"session_id": "...", "round_index": 0, "block_index": 2, "role": "thinking", "text": "chain of thought"}
{"session_id": "...", "round_index": 0, "block_index": 3, "role": "claude", "text": "response text"}
```

**Markdown format**: YAML frontmatter with session metadata, then selected blocks grouped by round:

```
---
session: <id>
project: /Users/frank/code/foo
exported_blocks: 12
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
```

Rounds that have no selected blocks are omitted entirely.

## Theme

Color palette (matches [md2html](https://github.com/algonous/md2html)):

| Block    | Border    | Background | Role text |
|----------|-----------|------------|-----------|
| YOU      | `#d97706` | `#fefce8`  | `#854d0e` |
| CLAUDE   | `#3b82f6` | `#eff6ff`  | `#1e40af` |
| TOOL     | `#22c55e` | `#f0fdf4`  | `#166534` |
| CONTEXT  | `#a78bfa` | `#f5f3ff`  | `#5b21b6` |
| THINKING | `#9ca3af` | `#f9fafb`  | `#4b5563` |

Page background: `#f5f5f3`. Code blocks: `#f6f8fa` bg, `#e1e4e8` border.

### Client-side rendering features

**Round fold**: Each round header includes a fold arrow. Clicking the round header collapses or expands the round body (all blocks within that round). Default is open. When folded, a summary appears in the header showing the first block's text (~80 chars) plus the total block count (e.g. "Fix the login bug... (5 blocks)").

**Consecutive block grouping**: Runs of 2+ consecutive blocks with the same role are wrapped in a collapsible `block-group` container. The group header shows a fold arrow, the role label, the count, and a fold summary. Group fold defaults match block fold defaults: TOOL/THINKING/CONTEXT groups start folded, YOU/CLAUDE groups start open. Each inner block retains its own individual fold. Group fold summary for tool groups lists unique tool names (e.g. "Read, Edit, Bash"); for other roles it shows "N blocks". Runs of 1 block are rendered directly without a group wrapper.

Grouping algorithm: iterate blocks in order, extending the current run when the role matches, otherwise starting a new run:

```
[you, thinking, claude, tool, tool, tool, claude]
-> [{role:"you", count:1}, {role:"thinking", count:1}, {role:"claude", count:1},
    {role:"tool", count:3}, {role:"claude", count:1}]
```

**Round sort toggle**: A toolbar button ("Oldest first" / "Newest first") toggles round display order between ascending (chronological) and descending (newest first). State is kept in `state.roundOrder` ('asc' or 'desc'). Rendering builds an index array and reverses it for desc order. The original round index is preserved for block IDs and anchor links. During polling, new rounds are inserted at the top when desc, at the bottom when asc.

**Hover anchor permalinks**: A `#` icon appears on hover to the right of round headers and block role labels. Clicking it copies the permalink URL to clipboard and shows a status message. Click events stop propagation to prevent fold toggling. URL format follows the routing spec: `/<sessionId>/<roundIdx>` for rounds, `/<sessionId>/<roundIdx>/<blockIdx>` for blocks.
