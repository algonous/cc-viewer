package tui

import "github.com/charmbracelet/lipgloss"

const sidebarWidth = 40

var (
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Viewer styles.
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

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("237"))

	headerActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255"))

	headerInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)
