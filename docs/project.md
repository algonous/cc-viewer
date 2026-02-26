# cc-viewer: Claude Code Session Browser

## Overview

A web-based application that browses Claude Code conversation history stored
locally under `~/.claude/`. Built with Go, serves an embedded web UI from a
single binary.

## Features

- Browse all Claude Code sessions with timestamps and project info
- View full transcripts organized by rounds (user -> tools -> thinking -> assistant)
- Fold/unfold blocks (context, tool, thinking folded by default)
- Token usage tracking per round (input, output, cache read, cache creation)
- Select blocks with checkboxes, copy text to clipboard
- Export sessions to JSONL or Markdown (downloads to browser)
- Filter sessions by project or content
- URL routing: `/<sessionId>` links directly to a session

## Architecture

- `internal/data/` -- data parsing layer (history index, transcript parsing, export generation)
- `internal/server/` -- HTTP handlers, md2html rendering, JSON API
- `web/` -- embedded SPA (index.html, app.css, app.js)
- `main.go` -- entry point, go:embed, CLI flags

## Dependencies

- [md2html](https://github.com/algonous/md2html) -- markdown to HTML pipeline (AST -> IR -> HTML)
- Go stdlib `net/http` -- HTTP server (Go 1.22+ ServeMux)
- No frontend dependencies (vanilla CSS + JS)
