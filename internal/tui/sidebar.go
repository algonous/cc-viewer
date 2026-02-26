package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/kfu/cc-tree/internal/data"
)

// sidebarModel manages the session list in the left panel.
type sidebarModel struct {
	sessions   []data.SessionSummary
	filtered   []int // indices into sessions
	cursor     int
	offset     int // scroll offset
	height     int
	filterText string
	filtering  bool
}

func newSidebar(sessions []data.SessionSummary) sidebarModel {
	s := sidebarModel{
		sessions: sessions,
	}
	s.applyFilter("")
	return s
}

func (s *sidebarModel) applyFilter(text string) {
	s.filterText = text
	s.filtered = s.filtered[:0]
	lower := strings.ToLower(text)
	for i, sess := range s.sessions {
		if text == "" ||
			strings.Contains(strings.ToLower(sess.ProjectName), lower) ||
			strings.Contains(strings.ToLower(sess.FirstMessage), lower) ||
			strings.Contains(strings.ToLower(sess.Project), lower) {
			s.filtered = append(s.filtered, i)
		}
	}
	s.cursor = 0
	s.offset = 0
}

func (s *sidebarModel) moveUp() {
	if s.cursor > 0 {
		s.cursor--
		if s.cursor < s.offset {
			s.offset = s.cursor
		}
	}
}

func (s *sidebarModel) moveDown() {
	if s.cursor < len(s.filtered)-1 {
		s.cursor++
		visible := s.visibleCount()
		if s.cursor >= s.offset+visible {
			s.offset = s.cursor - visible + 1
		}
	}
}

func (s *sidebarModel) visibleCount() int {
	// Each item takes 2 lines + 1 blank = 3 lines.
	if s.height <= 0 {
		return 10
	}
	n := s.height / 3
	if n < 1 {
		n = 1
	}
	return n
}

func (s *sidebarModel) selected() *data.SessionSummary {
	if len(s.filtered) == 0 {
		return nil
	}
	idx := s.filtered[s.cursor]
	return &s.sessions[idx]
}

// viewLines returns exactly `height` lines of sidebar content, each padded to sidebarWidth.
func (s *sidebarModel) viewLines(focused bool) []string {
	var lines []string

	if s.filtering {
		lines = append(lines, padRight("Filter: "+s.filterText+"_", sidebarWidth))
		lines = append(lines, padRight("", sidebarWidth))
	}

	visible := s.visibleCount()
	end := s.offset + visible
	if end > len(s.filtered) {
		end = len(s.filtered)
	}

	for vi := s.offset; vi < end; vi++ {
		idx := s.filtered[vi]
		sess := s.sessions[idx]
		isCurrent := vi == s.cursor

		ts := time.UnixMilli(sess.LastTS).Format("01/02 15:04")
		label := fmt.Sprintf("%-16s %s", truncate(sess.ProjectName, 16), ts)
		msg := truncate(sess.FirstMessage, sidebarWidth-5)

		line1 := padRight("  "+label, sidebarWidth)
		line2 := padRight("  \""+msg+"\"", sidebarWidth)
		if isCurrent {
			line1 = padRight("> "+label, sidebarWidth)
		}

		if isCurrent && focused {
			lines = append(lines, selectedStyle.Render(line1))
			lines = append(lines, selectedStyle.Render(line2))
		} else if isCurrent {
			lines = append(lines, line1)
			lines = append(lines, line2)
		} else {
			lines = append(lines, dimStyle.Render(line1))
			lines = append(lines, dimStyle.Render(line2))
		}

		if vi < end-1 {
			lines = append(lines, padRight("", sidebarWidth))
		}
	}

	if len(s.filtered) == 0 {
		lines = append(lines, dimStyle.Render(padRight("  (no sessions)", sidebarWidth)))
	}

	// Pad to exactly height lines.
	for len(lines) < s.height {
		lines = append(lines, padRight("", sidebarWidth))
	}
	// Truncate if somehow over.
	if len(lines) > s.height {
		lines = lines[:s.height]
	}

	return lines
}

func truncate(s string, max int) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
