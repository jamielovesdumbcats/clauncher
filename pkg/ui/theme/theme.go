package theme

import (
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Primary    lipgloss.Style
	Secondary  lipgloss.Style
	Success    lipgloss.Style
	Error      lipgloss.Style
	Warning    lipgloss.Style
	Info       lipgloss.Style
	Header     lipgloss.Style
	Footer     lipgloss.Style
	Border     lipgloss.Style
	Dim        lipgloss.Style
	Faint      lipgloss.Style
}

func NewTheme() *Theme {
	return &Theme{
		Primary:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")), // Blue
		Secondary:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // Cyan
		Success:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // Green
		Error:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")), // Red
		Warning:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // Yellow
		Info:       lipgloss.NewStyle().Foreground(lipgloss.Color("8")), // Gray
		Header:     lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		Footer:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")),
		Dim:        lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Faint:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}
