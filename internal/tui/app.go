package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
		sidebarWidth := 42 // 40 + 2 for border
		viewerWidth := m.width - sidebarWidth
		viewerHeight := m.height - 2 // status bar
		m.sidebar.height = viewerHeight
		m.viewer.setSize(viewerWidth, viewerHeight)
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

	case msg.String() == "/":
		m.sidebar.filtering = true
		m.sidebar.filterText = ""
		return m, nil

	case msg.String() == "d":
		return m, m.exportSelected()
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

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	sidebarContent := m.sidebar.view(m.focus == focusSidebar)
	sidebarRendered := sidebarStyle.Height(m.height - 2).Render(sidebarContent)

	viewerContent := m.viewer.view()
	viewerRendered := viewerStyle.Render(viewerContent)

	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebarRendered, viewerRendered)

	status := statusStyle.Render("Tab:switch  j/k:navigate  d:export  /:filter  q:quit")
	if m.status != "" {
		status = statusStyle.Render(m.status + "  |  Tab:switch  j/k:navigate  d:export  /:filter  q:quit")
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}
