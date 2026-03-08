package main

import (
	"embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	claudeDir := flag.String("claude-dir", "", "legacy single data directory (for compatibility)")
	dataDirs := flag.String("data-dirs", "", "comma-separated data roots; if omitted, auto-discovers all supported agent roots under $HOME")
	addr := flag.String("addr", "127.0.0.1:0", "listen address (host:port, use port 0 for auto)")
	flag.Parse()

	roots := make([]data.SourceRoot, 0, 4)
	if *claudeDir != "" {
		source := data.DetectSourceFromDir(*claudeDir)
		if source == "" {
			source = data.SourceClaude
		}
		roots = append(roots, data.SourceRoot{Source: source, Dir: *claudeDir})
	} else if strings.TrimSpace(*dataDirs) != "" {
		for _, raw := range strings.Split(*dataDirs, ",") {
			dir := strings.TrimSpace(raw)
			if dir == "" {
				continue
			}
			if strings.HasPrefix(dir, "~"+string(os.PathSeparator)) {
				dir = filepath.Join(home, dir[2:])
			}
			source := data.DetectSourceFromDir(dir)
			if source != "" {
				roots = append(roots, data.SourceRoot{Source: source, Dir: dir})
			}
		}
	} else {
		roots = data.DiscoverDefaultRoots(home)
	}

	sessions, err := data.LoadSessionsMulti(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		os.Exit(0)
	}

	srv := server.New(roots, sessions, webFS)
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
