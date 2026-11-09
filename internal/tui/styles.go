package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorOk   = lipgloss.Color("#3DDB8A")
	colorWarn = lipgloss.Color("#E6C35C")
	colorFail = lipgloss.Color("#F07178")
	colorMute = lipgloss.Color("#6B7280")
	colorAcc  = lipgloss.Color("#5CC8FF")
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAcc).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMute)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E6EDF3")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorMute).
			MarginBottom(1)

	okStyle   = lipgloss.NewStyle().Foreground(colorOk).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	failStyle = lipgloss.NewStyle().Foreground(colorFail).Bold(true)
	muteStyle = lipgloss.NewStyle().Foreground(colorMute)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorAcc).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().Foreground(colorMute).MarginTop(1)
)
