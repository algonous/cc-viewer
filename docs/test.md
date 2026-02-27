# Testing

## URL-addressable UI

Every visible state has a URL, making the UI testable via HTTP requests without a browser:

```
/                                        -> session list (first session selected)
/<sessionId>                             -> open session, viewer starts streaming
/<sessionId>/<roundIdx>                  -> open session, scroll to round
/<sessionId>/<roundIdx>/<blockIdx>       -> open session, scroll to round and block
```

Round and block indices are stable -- the transcript file is append-only, so existing indices never change. New rounds/blocks only get higher indices.

Actions (export, fold, select) are not URL-addressable -- they are either API calls or ephemeral frontend state.

## Test layers

The three-layer architecture allows each layer to be tested independently with Go stdlib `testing`.

### File tail layer

Create temp files, append lines programmatically, verify emitted events.

- Emits complete JSONL lines only (no partial lines)
- Buffers incomplete trailing data until next `\n`
- Handles empty files (no events until content appears)
- Handles non-existent files (waits for file to appear)
- Emits new lines when file grows (simulate Claude Code appending)

### Event processing layer

Feed mock JSONL strings, verify output events.

**Session events** (from history.jsonl lines):

- New sessionId -> creates session summary with correct project, timestamp, first message
- Multiple entries with same sessionId -> grouped into one session
- `"display": "exit"` -> session marked as exited
- Exit then resume (same sessionId, user message after exit) -> session marked as not exited
- Orphan transcript files (not in history.jsonl) -> discovered, treated as exited

**Block events** (from transcript lines):

- `user` with string content -> YOU block, starts new round
- `user` with array content -> skipped (tool_result)
- `assistant` with `text` block -> CLAUDE block
- `assistant` with `thinking` block -> THINKING block
- `assistant` with `tool_use` block -> TOOL block
- System-injected context messages -> CONTEXT block instead of YOU
- Multiple assistant entries in same round -> blocks emitted in file order
- Usage aggregation: output_tokens summed, input_tokens/cache max'd

**Lightweight scan** (startup search index):

- Extracts user message text
- Extracts assistant text blocks
- Skips thinking and tool_use content
- Does not render markdown

### Web layer (SSE)

Use Go `httptest` to start a test server, connect as SSE client, verify event stream.

**Session stream**:

- Initial connection receives all existing sessions
- Appending to history.jsonl -> client receives new session event
- Appending exit entry -> client receives exit status update

**Transcript stream**:

- Opening `GET /api/transcript/{id}` streams existing blocks as SSE events
- Each SSE event is valid JSON with role, round_index, and html (or tool data)
- Appending to transcript file -> client receives new block events
- Blocks arrive in chronological order (file order)
- Markdown content is rendered to HTML in the event payload
- Closing connection stops the tail

## Test fixtures

Tests use a temp directory that mirrors `~/.claude/` structure:

```
testdata/
  history.jsonl                    # pre-built with known sessions
  projects/
    -test-project/
      session-001.jsonl            # small transcript with all block types
      session-002.jsonl            # empty (for testing tail-wait behavior)
```

Fixtures contain minimal but representative data: a session with user messages, thinking, tool_use, text blocks, usage entries, and an exit marker.

## What is NOT tested automatically

- Visual rendering (CSS layout, colors, fold animations)
- Clipboard operations (copy to clipboard)
- File download (export button triggering browser download)

These require manual verification or a headless browser, which is outside the current scope.
