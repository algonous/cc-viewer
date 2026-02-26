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
	session  *data.SessionSummary
	rounds   []data.Round
	ready    bool
}

func newViewer() viewerModel {
	return viewerModel{}
}

func (v *viewerModel) setSize(width, height int) {
	if !v.ready {
		v.viewport = viewport.New(width, height)
		v.viewport.SetContent(v.content)
		v.ready = true
	} else {
		v.viewport.Width = width
		v.viewport.Height = height
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
			b.WriteString(userLabelStyle.Render("[YOU]") + " " + r.UserMessage + "\n\n")
		}

		for _, tc := range r.ToolCalls {
			line := fmt.Sprintf("[TOOL] %s: %s", tc.Name, tc.InputSummary)
			b.WriteString(toolLabelStyle.Render(line) + "\n")
		}
		if len(r.ToolCalls) > 0 {
			b.WriteString("\n")
		}

		for _, text := range r.AssistantTexts {
			b.WriteString(claudeLabelStyle.Render("[CLAUDE]") + " " + text + "\n\n")
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
