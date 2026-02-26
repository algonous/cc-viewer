# cc-viewer: Claude Code Session Browser

## Overview

A TUI application that browses Claude Code conversation history stored locally
under `~/.claude/`. Built with Go and Bubble Tea.

## Features

- Browse all Claude Code sessions with timestamps and project info
- View full transcripts organized by rounds (user message -> tool calls -> assistant response)
- Token usage tracking per round (input, output, cache read, cache creation)
- Export sessions to structured JSONL for analysis
- Filter sessions by project or content

## Architecture

- `internal/data/` -- data parsing layer (history index, transcript parsing, export)
- `internal/tui/` -- Bubble Tea TUI (sidebar list, transcript viewer, key bindings)
- `main.go` -- entry point, CLI flags, app initialization
