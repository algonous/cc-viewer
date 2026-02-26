# cc-tree

TUI session browser for Claude Code conversation history.

Browse, inspect, and export your Claude Code sessions with token/cost data.

## Install

```
make
```

Builds to `~/.local/bin/cc-tree` by default. Override with `BIN=/your/path make`.

## Usage

```
cc-tree [--claude-dir ~/.claude]
```

### Key bindings

| Key   | Action                     |
|-------|----------------------------|
| Up/Down | Navigate sessions        |
| Enter | Select session             |
| Tab   | Switch focus (sidebar/viewer) |
| d     | Export selected session     |
| /     | Filter sessions            |
| q     | Quit                       |

## Data sources

Reads from `~/.claude/`:
- `history.jsonl` -- session index
- `projects/<encoded-dir>/<sessionId>.jsonl` -- full transcripts

## Export

Press `d` to export the selected session to `~/.config/cc-tree/exports/<sessionId>.jsonl`.
One line per round with user message, tool calls, assistant response, and token usage.
