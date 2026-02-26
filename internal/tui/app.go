package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kfu/cc-tree/internal/data"
)

// focus tracks which panel has focus.
type focus int

const (
	focusSidebar focus = iota
	focusViewer
)

// transcriptLoadedMsg is sent when a transcript finishes loading.
type transcriptLoadedMsg struct {
	session    *data.SessionSummary
	transcript *data.Transcript
	err        error
}

// exportDoneMsg is sent when an export completes.
type exportDoneMsg struct {
	path string
	err  error
}

// Model is the root Bubble Tea model.
type Model struct {
	claudeDir string
	sidebar   sidebarModel
	viewer    viewerModel
	focus     focus
	width     int
	height    int
	status    string
}

// New creates a new root model.
func New(claudeDir string, sessions []data.SessionSummary) Model {
	m := Model{
		claudeDir: claudeDir,
		sidebar:   newSidebar(sessions),
		viewer:    newViewer(),
		focus:     focusSidebar,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	// Auto-select the first session if available.
	if sess := m.sidebar.selected(); sess != nil {
		return m.loadTranscript(sess)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := m.height - 3 // header + border + status bar
		viewerWidth := m.width - sidebarWidth - 1 // 1 for separator
		m.sidebar.height = contentHeight
		m.viewer.setSize(viewerWidth, contentHeight)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case transcriptLoadedMsg:
		if msg.err != nil {
			m.viewer.setError(fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.viewer.setTranscript(msg.session, msg.transcript)
		}
		m.status = ""
		return m, nil

	case exportDoneMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Export failed: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("Exported to %s", msg.path)
		}
		return m, nil
	}

	if m.focus == focusViewer {
		cmd := m.viewer.update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter mode input.
	if m.sidebar.filtering {
		switch msg.String() {
		case "esc":
			m.sidebar.filtering = false
			m.sidebar.applyFilter("")
			return m, nil
		case "enter":
			m.sidebar.filtering = false
			return m, nil
		case "backspace":
			if len(m.sidebar.filterText) > 0 {
				m.sidebar.filterText = m.sidebar.filterText[:len(m.sidebar.filterText)-1]
				m.sidebar.applyFilter(m.sidebar.filterText)
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.sidebar.filterText += msg.String()
				m.sidebar.applyFilter(m.sidebar.filterText)
			}
			return m, nil
		}
	}

	switch {
	case msg.String() == "q" || msg.String() == "ctrl+c":
		return m, tea.Quit

	case msg.String() == "tab":
		if m.focus == focusSidebar {
			m.focus = focusViewer
		} else {
			m.focus = focusSidebar
		}
		return m, nil

	case msg.String() == "l" && m.focus == focusSidebar:
		m.focus = focusViewer
		return m, nil

	case msg.String() == "h" && m.focus == focusViewer:
		m.focus = focusSidebar
		return m, nil

	case msg.String() == "/":
		m.sidebar.filtering = true
		m.sidebar.filterText = ""
		return m, nil

	case msg.String() == "d":
		if m.viewer.selectMode && m.viewer.selectedCount() > 0 {
			return m, m.exportSelectedRounds()
		}
		return m, m.exportSelected()
	}

	// Select mode in viewer.
	if m.focus == focusViewer && m.viewer.selectMode {
		switch msg.String() {
		case "esc":
			m.viewer.exitSelectMode()
			return m, nil
		case "j", "down":
			m.viewer.moveCursorDown()
			return m, nil
		case "k", "up":
			m.viewer.moveCursorUp()
			return m, nil
		case " ":
			m.viewer.toggleSelection()
			return m, nil
		}
		return m, nil
	}

	if m.focus == focusSidebar {
		switch {
		case msg.String() == "up" || msg.String() == "k":
			m.sidebar.moveUp()
			return m, m.selectCurrent()
		case msg.String() == "down" || msg.String() == "j":
			m.sidebar.moveDown()
			return m, m.selectCurrent()
		case msg.String() == "enter":
			return m, m.selectCurrent()
		}
	}

	if m.focus == focusViewer {
		switch msg.String() {
		case "s":
			m.viewer.enterSelectMode()
			return m, nil
		}
		cmd := m.viewer.update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) selectCurrent() tea.Cmd {
	sess := m.sidebar.selected()
	if sess == nil {
		return nil
	}
	m.status = "Loading..."
	return m.loadTranscript(sess)
}

func (m *Model) loadTranscript(sess *data.SessionSummary) tea.Cmd {
	claudeDir := m.claudeDir
	s := *sess
	return func() tea.Msg {
		path, err := data.FindTranscriptPath(claudeDir, s.SessionID)
		if err != nil {
			return transcriptLoadedMsg{session: &s, err: err}
		}
		t, err := data.LoadTranscript(path)
		return transcriptLoadedMsg{session: &s, transcript: t, err: err}
	}
}

func (m *Model) exportSelected() tea.Cmd {
	sess := m.sidebar.selected()
	if sess == nil {
		m.status = "No session selected"
		return nil
	}
	claudeDir := m.claudeDir
	s := *sess
	return func() tea.Msg {
		path, err := data.FindTranscriptPath(claudeDir, s.SessionID)
		if err != nil {
			return exportDoneMsg{err: err}
		}
		t, err := data.LoadTranscript(path)
		if err != nil {
			return exportDoneMsg{err: err}
		}
		outPath, err := data.ExportSession(data.ConfigDir(), s, t)
		return exportDoneMsg{path: outPath, err: err}
	}
}

func (m *Model) exportSelectedRounds() tea.Cmd {
	sess := m.sidebar.selected()
	if sess == nil {
		m.status = "No session selected"
		return nil
	}
	rounds := m.viewer.selectedRounds()
	if len(rounds) == 0 {
		m.status = "No rounds selected"
		return nil
	}
	s := *sess
	r := make([]data.Round, len(rounds))
	copy(r, rounds)
	return func() tea.Msg {
		t := &data.Transcript{SessionID: s.SessionID, Rounds: r}
		outPath, err := data.ExportSession(data.ConfigDir(), s, t)
		return exportDoneMsg{path: outPath, err: err}
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	contentHeight := m.height - 3 // header + border + status bar
	sep := separatorStyle.Render("|")

	// Header row -- session ID left, summary right.
	sidebarHeader := " Sessions"
	if sess := m.sidebar.selected(); sess != nil {
		sidebarHeader = " " + sess.SessionID
	}
	sidebarHeader = padRight(sidebarHeader, sidebarWidth)
	viewerHeader := " " + m.viewer.summaryLine()

	var b strings.Builder
	if m.focus == focusSidebar {
		b.WriteString(headerActiveStyle.Render(sidebarHeader))
		b.WriteString(sep)
		b.WriteString(headerInactiveStyle.Render(viewerHeader))
	} else {
		b.WriteString(headerInactiveStyle.Render(sidebarHeader))
		b.WriteString(sep)
		b.WriteString(headerActiveStyle.Render(viewerHeader))
	}
	b.WriteString("\n")

	// Border line under header.
	viewerWidth := m.width - sidebarWidth - 1
	b.WriteString(padRight("", sidebarWidth))
	b.WriteString(sep)
	b.WriteString(separatorStyle.Render(strings.Repeat("-", viewerWidth)))
	b.WriteString("\n")

	sidebarLines := m.sidebar.viewLines(m.focus == focusSidebar)
	viewerLines := strings.Split(m.viewer.view(), "\n")

	for len(viewerLines) < contentHeight {
		viewerLines = append(viewerLines, "")
	}

	for i := 0; i < contentHeight; i++ {
		sl := ""
		if i < len(sidebarLines) {
			sl = sidebarLines[i]
		}
		vl := ""
		if i < len(viewerLines) {
			vl = viewerLines[i]
		}
		b.WriteString(sl)
		b.WriteString(sep)
		b.WriteString(" ")
		b.WriteString(vl)
		b.WriteString("\n")
	}

	var status string
	if m.viewer.selectMode {
		n := m.viewer.selectedCount()
		status = fmt.Sprintf("SELECT: j/k:move  space:toggle  d:export(%d)  esc:cancel", n)
	} else {
		status = "h/l:switch  j/k:navigate  s:select  d:export  /:filter  q:quit"
	}
	if m.status != "" {
		status = m.status + "  |  " + status
	}
	b.WriteString(statusStyle.Render(status))

	return b.String()
}
