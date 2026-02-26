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
	viewport   viewport.Model
	content    string
	width      int
	session    *data.SessionSummary
	rounds     []data.Round
	ready      bool
	selectMode bool
	cursor     int           // current round index in select mode
	selected   map[int]bool  // selected round indices
}

func newViewer() viewerModel {
	return viewerModel{
		selected: make(map[int]bool),
	}
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
	v.selectMode = false
	v.cursor = 0
	v.selected = make(map[int]bool)
	v.content = v.renderTranscript()
	if v.ready {
		v.viewport.SetContent(v.content)
		v.viewport.GotoTop()
	}
}

func (v *viewerModel) setError(msg string) {
	v.session = nil
	v.rounds = nil
	v.selectMode = false
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

// enterSelectMode switches to block-navigation select mode.
func (v *viewerModel) enterSelectMode() {
	if len(v.rounds) == 0 {
		return
	}
	v.selectMode = true
	v.cursor = 0
	v.selected = make(map[int]bool)
	v.rerender()
	v.scrollToCursor()
}

// exitSelectMode returns to normal scroll mode.
func (v *viewerModel) exitSelectMode() {
	v.selectMode = false
	v.selected = make(map[int]bool)
	v.rerender()
}

// moveCursorUp moves the block cursor up one round.
func (v *viewerModel) moveCursorUp() {
	if v.cursor > 0 {
		v.cursor--
		v.rerender()
		v.scrollToCursor()
	}
}

// moveCursorDown moves the block cursor down one round.
func (v *viewerModel) moveCursorDown() {
	if v.cursor < len(v.rounds)-1 {
		v.cursor++
		v.rerender()
		v.scrollToCursor()
	}
}

// toggleSelection toggles the current round's selection.
func (v *viewerModel) toggleSelection() {
	if v.selected[v.cursor] {
		delete(v.selected, v.cursor)
	} else {
		v.selected[v.cursor] = true
	}
	v.rerender()
}

// selectedCount returns how many rounds are selected.
func (v *viewerModel) selectedCount() int {
	return len(v.selected)
}

// selectedRounds returns the rounds that are selected, in order.
func (v *viewerModel) selectedRounds() []data.Round {
	var result []data.Round
	for i, r := range v.rounds {
		if v.selected[i] {
			result = append(result, r)
		}
	}
	return result
}

// summaryLine returns a one-line summary of the loaded transcript.
func (v *viewerModel) summaryLine() string {
	if v.session == nil || len(v.rounds) == 0 {
		return "Transcript"
	}
	var total data.Usage
	for _, r := range v.rounds {
		total.InputTokens += r.Usage.InputTokens
		total.OutputTokens += r.Usage.OutputTokens
		total.CacheRead += r.Usage.CacheRead
		total.CacheCreation += r.Usage.CacheCreation
	}
	return fmt.Sprintf("%d rounds | in=%d out=%d cache_read=%d cache_write=%d",
		len(v.rounds), total.InputTokens, total.OutputTokens,
		total.CacheRead, total.CacheCreation)
}

func (v *viewerModel) rerender() {
	v.content = v.renderTranscript()
	if v.ready {
		v.viewport.SetContent(v.content)
	}
}

// scrollToCursor scrolls the viewport so the current round is visible.
func (v *viewerModel) scrollToCursor() {
	if !v.ready || len(v.rounds) == 0 {
		return
	}
	// Find the line offset of the cursor round by counting lines in rendered content.
	lines := strings.Split(v.content, "\n")
	roundIdx := 0
	targetLine := 0
	for i, line := range lines {
		if strings.Contains(line, fmt.Sprintf("Round %d", roundIdx+1)) && strings.Contains(line, "---") {
			if roundIdx == v.cursor {
				targetLine = i
				break
			}
			roundIdx++
		}
	}
	// Center the target in the viewport if possible.
	offset := targetLine - v.viewport.Height/3
	if offset < 0 {
		offset = 0
	}
	maxOffset := len(lines) - v.viewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	v.viewport.SetYOffset(offset)
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

	for i, r := range v.rounds {
		isCursor := v.selectMode && i == v.cursor
		isSelected := v.selectMode && v.selected[i]
		isDimmed := v.selectMode && !isCursor

		block := v.renderRound(r, w, isCursor, isSelected)
		if isDimmed {
			// Dim every line of the block.
			var dimmed strings.Builder
			for j, line := range strings.Split(block, "\n") {
				if j > 0 {
					dimmed.WriteByte('\n')
				}
				dimmed.WriteString(dimStyle.Render(stripAnsi(line)))
			}
			b.WriteString(dimmed.String())
		} else {
			b.WriteString(block)
		}

		totalUsage.InputTokens += r.Usage.InputTokens
		totalUsage.OutputTokens += r.Usage.OutputTokens
		totalUsage.CacheRead += r.Usage.CacheRead
		totalUsage.CacheCreation += r.Usage.CacheCreation
	}

	return b.String()
}

// renderRound renders a single round block as a styled string.
func (v *viewerModel) renderRound(r data.Round, w int, isCursor, isSelected bool) string {
	var rb strings.Builder

	marker := ""
	if v.selectMode {
		if isSelected {
			marker = "[x] "
		} else {
			marker = "[ ] "
		}
	}

	header := fmt.Sprintf("--- %sRound %d", marker, r.Index+1)
	if r.UserTimestamp != "" {
		header += " (" + r.UserTimestamp + ")"
	}
	header += " ---"

	if isCursor {
		rb.WriteString(selectedStyle.Render(header) + "\n\n")
	} else {
		rb.WriteString(roundHeaderStyle.Render(header) + "\n\n")
	}

	if r.UserMessage != "" {
		if r.IsContext {
			summary := contextSummary(r.UserMessage)
			rb.WriteString(dimStyle.Render("[CONTEXT] "+summary) + "\n\n")
		} else {
			wrapped := wrapText(r.UserMessage, w-6)
			lines := strings.Split(wrapped, "\n")
			rb.WriteString(userLabelStyle.Render("[YOU]") + " " + lines[0] + "\n")
			for _, l := range lines[1:] {
				rb.WriteString("      " + l + "\n")
			}
			rb.WriteString("\n")
		}
	}

	for _, tc := range r.ToolCalls {
		line := fmt.Sprintf("[TOOL] %s: %s", tc.Name, tc.InputSummary)
		rb.WriteString(toolLabelStyle.Render(wrapText(line, w)) + "\n")
	}
	if len(r.ToolCalls) > 0 {
		rb.WriteString("\n")
	}

	for _, text := range r.AssistantTexts {
		wrapped := wrapText(text, w-9)
		lines := strings.Split(wrapped, "\n")
		rb.WriteString(claudeLabelStyle.Render("[CLAUDE]") + " " + lines[0] + "\n")
		for _, l := range lines[1:] {
			rb.WriteString("         " + l + "\n")
		}
		rb.WriteString("\n")
	}

	if r.Usage.OutputTokens > 0 || r.Usage.InputTokens > 0 {
		usage := fmt.Sprintf("  Tokens: in=%d out=%d cache_read=%d cache_write=%d",
			r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheRead, r.Usage.CacheCreation)
		rb.WriteString(usageStyle.Render(usage) + "\n")
	}
	rb.WriteString("\n")

	return rb.String()
}

// stripAnsi removes ANSI escape sequences from a string so it can be
// re-styled uniformly (e.g. dimmed).
func stripAnsi(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm' (end of SGR sequence).
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
			} else {
				i = j
			}
		} else {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

// wrapText hard-wraps text to fit within the given width.
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
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return "(plan: " + strings.TrimPrefix(line, "# ") + ")"
			}
		}
		return "(plan implementation)"
	}
	first := strings.SplitN(text, "\n", 2)[0]
	if len(first) > 80 {
		first = first[:77] + "..."
	}
	return "(" + first + ")"
}
