package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kfu/cc-tree/internal/data"
)

// viewerModel manages the transcript display in the right panel.
type viewerModel struct {
	viewport viewport.Model
	content  string
	width    int
	session  *data.SessionSummary
	rounds   []data.Round
	ready    bool
}

func newViewer() viewerModel {
	return viewerModel{}
}

func (v *viewerModel) setSize(width, height int) {
	v.width = width
	if !v.ready {
		v.viewport = viewport.New(width, height)
		v.viewport.SetContent(v.content)
		v.ready = true
	} else {
		v.viewport.Width = width
		v.viewport.Height = height
		// Re-render with new width for wrapping.
		if v.session != nil {
			v.content = v.renderTranscript()
			v.viewport.SetContent(v.content)
		}
	}
}

func (v *viewerModel) setTranscript(session *data.SessionSummary, t *data.Transcript) {
	v.session = session
	if t != nil {
		v.rounds = t.Rounds
	} else {
		v.rounds = nil
	}
	v.content = v.renderTranscript()
	if v.ready {
		v.viewport.SetContent(v.content)
		v.viewport.GotoTop()
	}
}

func (v *viewerModel) setError(msg string) {
	v.session = nil
	v.rounds = nil
	v.content = msg
	if v.ready {
		v.viewport.SetContent(v.content)
		v.viewport.GotoTop()
	}
}

func (v *viewerModel) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.viewport, cmd = v.viewport.Update(msg)
	return cmd
}

func (v *viewerModel) view() string {
	if !v.ready {
		return "Loading..."
	}
	return v.viewport.View()
}

func (v *viewerModel) renderTranscript() string {
	if v.session == nil || len(v.rounds) == 0 {
		return dimStyle.Render("Select a session to view")
	}

	w := v.width
	if w <= 0 {
		w = 80
	}

	var b strings.Builder
	var totalUsage data.Usage

	for _, r := range v.rounds {
		header := fmt.Sprintf("--- Round %d", r.Index+1)
		if r.UserTimestamp != "" {
			header += " (" + r.UserTimestamp + ")"
		}
		header += " ---"
		b.WriteString(roundHeaderStyle.Render(header) + "\n\n")

		if r.UserMessage != "" {
			if r.IsContext {
				summary := contextSummary(r.UserMessage)
				b.WriteString(dimStyle.Render("[CONTEXT] "+summary) + "\n\n")
			} else {
				wrapped := wrapText(r.UserMessage, w-6) // 6 = "[YOU] " prefix
				lines := strings.Split(wrapped, "\n")
				b.WriteString(userLabelStyle.Render("[YOU]") + " " + lines[0] + "\n")
				for _, l := range lines[1:] {
					b.WriteString("      " + l + "\n")
				}
				b.WriteString("\n")
			}
		}

		for _, tc := range r.ToolCalls {
			line := fmt.Sprintf("[TOOL] %s: %s", tc.Name, tc.InputSummary)
			b.WriteString(toolLabelStyle.Render(wrapText(line, w)) + "\n")
		}
		if len(r.ToolCalls) > 0 {
			b.WriteString("\n")
		}

		for _, text := range r.AssistantTexts {
			wrapped := wrapText(text, w-9) // 9 = "[CLAUDE] " prefix
			lines := strings.Split(wrapped, "\n")
			b.WriteString(claudeLabelStyle.Render("[CLAUDE]") + " " + lines[0] + "\n")
			for _, l := range lines[1:] {
				b.WriteString("         " + l + "\n")
			}
			b.WriteString("\n")
		}

		if r.Usage.OutputTokens > 0 || r.Usage.InputTokens > 0 {
			usage := fmt.Sprintf("  Tokens: in=%d out=%d cache=%d",
				r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheRead)
			b.WriteString(usageStyle.Render(usage) + "\n")
		}
		b.WriteString("\n")

		totalUsage.InputTokens += r.Usage.InputTokens
		totalUsage.OutputTokens += r.Usage.OutputTokens
		totalUsage.CacheRead += r.Usage.CacheRead
		totalUsage.CacheCreation += r.Usage.CacheCreation
	}

	b.WriteString(roundHeaderStyle.Render("--- Summary ---") + "\n")
	b.WriteString(fmt.Sprintf("  Rounds: %d\n", len(v.rounds)))
	b.WriteString(fmt.Sprintf("  Total tokens: in=%d out=%d cache_read=%d cache_create=%d\n",
		totalUsage.InputTokens, totalUsage.OutputTokens,
		totalUsage.CacheRead, totalUsage.CacheCreation))

	return b.String()
}

// wrapText hard-wraps text to fit within the given width.
// It wraps on word boundaries when possible.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		if len(paragraph) <= width {
			result.WriteString(paragraph)
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				result.WriteString(line)
				result.WriteByte('\n')
				line = word
			}
			// Handle single words longer than width.
			for len(line) > width {
				result.WriteString(line[:width])
				result.WriteByte('\n')
				line = line[width:]
			}
		}
		if line != "" {
			result.WriteString(line)
		}
	}
	return result.String()
}

// contextSummary returns a short description for system-injected context.
func contextSummary(text string) string {
	if strings.HasPrefix(text, "This session is being continued") {
		return "(session continuation summary)"
	}
	if strings.HasPrefix(text, "Implement the following plan:") {
		// Extract the plan title from the first heading.
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return "(plan: " + strings.TrimPrefix(line, "# ") + ")"
			}
		}
		return "(plan implementation)"
	}
	// Fallback: first line truncated.
	first := strings.SplitN(text, "\n", 2)[0]
	if len(first) > 80 {
		first = first[:77] + "..."
	}
	return "(" + first + ")"
}
