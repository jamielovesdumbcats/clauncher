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
	Panel      lipgloss.Style
	PanelTitle lipgloss.Style
	GPUPanel   lipgloss.Style
	Command    lipgloss.Style
	Key        lipgloss.Style
	Banner     lipgloss.Style
}

func NewTheme() *Theme {
	return &Theme{
		Primary:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		Secondary:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		Success:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		Error:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Warning:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		Info:       lipgloss.NewStyle().Foreground(lipgloss.Color("74")),
		Header:     lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).PaddingLeft(2),
		Footer:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")).PaddingLeft(2),
		Border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1),
		Dim:        lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		Faint:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("13")).
			Padding(0, 1).
			MarginTop(1),
		PanelTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).PaddingLeft(1),
		GPUPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("13")).
			Padding(0, 1).
			MarginTop(1).
			Width(30),
		Command:    lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true),
		Key:        lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		Banner:     lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true),
	}
}
