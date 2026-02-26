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
	// Each item takes 2 lines + 1 blank = 3 lines; last item no trailing blank.
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

func (s *sidebarModel) view(focused bool) string {
	var b strings.Builder

	if s.filtering {
		b.WriteString("Filter: " + s.filterText + "_\n\n")
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
		msg := truncate(sess.FirstMessage, 36)

		if isCurrent && focused {
			b.WriteString(selectedStyle.Render("> "+label) + "\n")
			b.WriteString(selectedStyle.Render("  \""+msg+"\"") + "\n")
		} else if isCurrent {
			b.WriteString("> " + label + "\n")
			b.WriteString("  \"" + msg + "\"\n")
		} else {
			b.WriteString(dimStyle.Render("  "+label) + "\n")
			b.WriteString(dimStyle.Render("  \""+msg+"\"") + "\n")
		}

		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if len(s.filtered) == 0 {
		b.WriteString(dimStyle.Render("  (no sessions)"))
	}

	return b.String()
}

func truncate(s string, max int) string {
	// Truncate to single line first.
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
