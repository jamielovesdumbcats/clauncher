package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette (256-color codes)
const (
	// Rainbow borders
	ColorCyan    = lipgloss.Color("45")  // GPU
	ColorMagenta = lipgloss.Color("213") // Models
	ColorYellow  = lipgloss.Color("220") // Servers
	ColorGreen   = lipgloss.Color("77")  // Commands
	ColorBlue    = lipgloss.Color("69")  // General panels
	ColorOrange  = lipgloss.Color("209") // Accent

	// Text
	ColorPrimary = lipgloss.Color("12")
	ColorSuccess = lipgloss.Color("10")
	ColorError   = lipgloss.Color("9")
	ColorWarning = lipgloss.Color("11")
	ColorInfo    = lipgloss.Color("74")
	ColorHeader  = lipgloss.Color("208")
	ColorFaint   = lipgloss.Color("240")
	ColorDim     = lipgloss.Color("243")
	ColorFooter  = lipgloss.Color("245")

	// Gradient accents
	ColorRed       = lipgloss.Color("196")
	ColorPink      = lipgloss.Color("212")
	ColorPurple    = lipgloss.Color("141")
	ColorLightBlue = lipgloss.Color("117")
	ColorMint      = lipgloss.Color("43")
	ColorLime      = lipgloss.Color("155")
)

type Theme struct {
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Info      lipgloss.Style
	Header    lipgloss.Style
	Footer    lipgloss.Style
	Faint     lipgloss.Style
	Dim       lipgloss.Style

	// Panels with distinct border colors
	Panel        lipgloss.Style
	PanelCyan    lipgloss.Style // GPU
	PanelMagenta lipgloss.Style // Models
	PanelYellow  lipgloss.Style // Servers
	PanelGreen   lipgloss.Style // Commands
	PanelBlue    lipgloss.Style // Launch options, benchmarks
	PanelOrange  lipgloss.Style // Search, catalog

	// Panel titles
	PanelTitle        lipgloss.Style
	PanelTitleCyan    lipgloss.Style
	PanelTitleMagenta lipgloss.Style
	PanelTitleYellow  lipgloss.Style

	Key    lipgloss.Style
	Banner lipgloss.Style
}

// basePanel creates a consistent rounded panel with given border color.
func basePanel(borderColor lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		MarginTop(1)
}

func NewTheme() *Theme {
	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		Success:   lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true),
		Error:     lipgloss.NewStyle().Foreground(ColorError).Bold(true),
		Warning:   lipgloss.NewStyle().Foreground(ColorWarning).Bold(true),
		Info:      lipgloss.NewStyle().Foreground(ColorInfo),
		Header:    lipgloss.NewStyle().Foreground(ColorHeader).Bold(true).PaddingLeft(2),
		Footer:    lipgloss.NewStyle().Foreground(ColorFooter).PaddingLeft(2),
		Faint:     lipgloss.NewStyle().Foreground(ColorFaint),
		Dim:       lipgloss.NewStyle().Foreground(ColorDim),

		// Default panel
		Panel: basePanel(ColorBlue),

		// Colored panels
		PanelCyan:    basePanel(ColorCyan),
		PanelMagenta: basePanel(ColorMagenta),
		PanelYellow:  basePanel(ColorYellow),
		PanelGreen:   basePanel(ColorGreen),
		PanelBlue:    basePanel(ColorBlue),
		PanelOrange:  basePanel(ColorOrange),

		// Titles
		PanelTitle:        lipgloss.NewStyle().Foreground(ColorHeader).Bold(true),
		PanelTitleCyan:    lipgloss.NewStyle().Foreground(ColorCyan).Bold(true),
		PanelTitleMagenta: lipgloss.NewStyle().Foreground(ColorMagenta).Bold(true),
		PanelTitleYellow:  lipgloss.NewStyle().Foreground(ColorYellow).Bold(true),

		Key:    lipgloss.NewStyle().Foreground(ColorHeader).Bold(true).PaddingLeft(1).PaddingRight(1),
		Banner: lipgloss.NewStyle().Foreground(ColorMagenta).Bold(true),
	}
}

// GradientBar renders a rainbow gradient bar across the terminal width.
func (t *Theme) GradientBar(width int) string {
	colors := []lipgloss.Color{
		ColorRed, ColorOrange, ColorYellow, ColorLime,
		ColorMint, ColorCyan, ColorLightBlue, ColorPurple, ColorPink,
	}
	var bar string
	chunk := max(1, width/len(colors))
	for _, c := range colors {
		for i := 0; i < chunk; i++ {
			bar += lipgloss.NewStyle().Foreground(c).Render("█")
		}
	}
	return bar
}

// PanelHeader renders a colored header line for a panel (top padding only).
func (t *Theme) PanelHeader(title string, titleStyle lipgloss.Style) string {
	return "\n" + titleStyle.Render("  "+title)
}

// StatusBar renders a bordered status bar with view context, key hints, and bottom gradient.
func (t *Theme) StatusBar(view string, hints []string, width int) string {
	// Build key hints
	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, t.Key.Render(h))
	}
	hintsText := lipgloss.JoinHorizontal(lipgloss.Top, hintParts...)

	// View indicator
	viewText := t.Faint.Render("  " + view)

	content := lipgloss.JoinHorizontal(lipgloss.Top, viewText, hintsText)

	barStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMint).
		Padding(0, 1)

	return "\n" + barStyle.Render(content) + "\n" + t.GradientBar(width)
}
