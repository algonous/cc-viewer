package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kfu/cc-tree/internal/data"
	"github.com/kfu/cc-tree/internal/tui"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	claudeDir := flag.String("claude-dir", filepath.Join(home, ".claude"), "path to Claude data directory")
	flag.Parse()

	sessions, err := data.LoadSessions(*claudeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		os.Exit(0)
	}

	m := tui.New(*claudeDir, sessions)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
