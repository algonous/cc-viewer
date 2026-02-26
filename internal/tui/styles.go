package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Sidebar styles.
	sidebarStyle = lipgloss.NewStyle().
			Width(40).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Viewer styles.
	viewerStyle = lipgloss.NewStyle().
			Padding(0, 1)

	roundHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("76"))

	toolLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	claudeLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("135"))

	usageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	// Status bar.
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)
)
