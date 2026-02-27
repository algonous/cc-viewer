package main

import (
	"embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/algonous/cc-viewer/internal/data"
	"github.com/algonous/cc-viewer/internal/server"
)

//go:embed web
var webFS embed.FS

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	claudeDir := flag.String("claude-dir", filepath.Join(home, ".claude"), "path to Claude data directory")
	addr := flag.String("addr", "127.0.0.1:0", "listen address (host:port, use port 0 for auto)")
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

	srv := server.New(*claudeDir, sessions, webFS)
	srv.StartHistoryTail()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://%s", ln.Addr().String())
	fmt.Printf("cc-viewer serving %d sessions at %s\n", len(sessions), url)

	if err := http.Serve(ln, srv.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
