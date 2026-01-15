// Package tui provides an interactive terminal user interface for poxy.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette - matches existing CLI colors
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Yellow
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorText      = lipgloss.Color("#F3F4F6") // Light gray
	ColorBg        = lipgloss.Color("#1F2937") // Dark gray
	ColorBgAlt     = lipgloss.Color("#374151") // Slightly lighter
)

// Source colors for different package managers
var SourceColors = map[string]lipgloss.Color{
	"pacman":  lipgloss.Color("#1793D1"), // Arch blue
	"apt":     lipgloss.Color("#A80030"), // Debian red
	"dnf":     lipgloss.Color("#294172"), // Fedora blue
	"brew":    lipgloss.Color("#FBB040"), // Homebrew yellow
	"flatpak": lipgloss.Color("#4A90D9"), // Flatpak blue
	"snap":    lipgloss.Color("#E95420"), // Ubuntu orange
	"aur":     lipgloss.Color("#1793D1"), // Arch blue
	"winget":  lipgloss.Color("#0078D4"), // Windows blue
}

// Styles contains all the lipgloss styles used in the TUI
type Styles struct {
	// App frame
	App       lipgloss.Style
	Header    lipgloss.Style
	Footer    lipgloss.Style
	StatusBar lipgloss.Style

	// Tabs
	Tab          lipgloss.Style
	TabActive    lipgloss.Style
	TabInactive  lipgloss.Style
	TabSeparator lipgloss.Style

	// Content
	Content     lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style

	// List items
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemDim      lipgloss.Style

	// Package display
	PackageName    lipgloss.Style
	PackageVersion lipgloss.Style
	PackageSource  lipgloss.Style
	PackageDesc    lipgloss.Style

	// Status indicators
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	// Input
	Input       lipgloss.Style
	InputPrompt lipgloss.Style
	InputCursor lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	HelpSep  lipgloss.Style

	// Spinner
	Spinner lipgloss.Style

	// Borders
	Border       lipgloss.Style
	BorderActive lipgloss.Style

	// Dialog
	Dialog       lipgloss.Style
	DialogTitle  lipgloss.Style
	DialogButton lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() *Styles {
	s := &Styles{}

	// App frame
	s.App = lipgloss.NewStyle().
		Background(ColorBg)

	s.Header = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorBgAlt).
		Padding(0, 1).
		Bold(true)

	s.Footer = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	s.StatusBar = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorBgAlt).
		Padding(0, 1)

	// Tabs
	s.Tab = lipgloss.NewStyle().
		Padding(0, 2)

	s.TabActive = s.Tab.
		Foreground(ColorPrimary).
		Bold(true).
		Underline(true)

	s.TabInactive = s.Tab.
		Foreground(ColorMuted)

	s.TabSeparator = lipgloss.NewStyle().
		Foreground(ColorMuted).
		SetString("|")

	// Content
	s.Content = lipgloss.NewStyle().
		Padding(1, 2)

	s.Title = lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	s.Description = lipgloss.NewStyle().
		Foreground(ColorMuted)

	// List items
	s.ListItem = lipgloss.NewStyle().
		PaddingLeft(2)

	s.ListItemSelected = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		PaddingLeft(0).
		SetString("> ")

	s.ListItemDim = lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingLeft(2)

	// Package display
	s.PackageName = lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true)

	s.PackageVersion = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	s.PackageSource = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Italic(true)

	s.PackageDesc = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(60)

	// Status indicators
	s.Success = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	s.Warning = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	s.Error = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	s.Info = lipgloss.NewStyle().
		Foreground(ColorSecondary)

	// Input
	s.Input = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorBgAlt).
		Padding(0, 1)

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	s.InputCursor = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	// Help
	s.HelpKey = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	s.HelpDesc = lipgloss.NewStyle().
		Foreground(ColorMuted)

	s.HelpSep = lipgloss.NewStyle().
		Foreground(ColorMuted).
		SetString(" - ")

	// Spinner
	s.Spinner = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	// Borders
	s.Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted)

	s.BorderActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary)

	// Dialog
	s.Dialog = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(60)

	s.DialogTitle = lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true).
		MarginBottom(1)

	s.DialogButton = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorPrimary).
		Padding(0, 2).
		MarginRight(1)

	return s
}

// SourceStyle returns a style for the given package source
func SourceStyle(source string) lipgloss.Style {
	color, ok := SourceColors[source]
	if !ok {
		color = ColorMuted
	}
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

// Badge creates a badge-style label
func Badge(text string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(color).
		Padding(0, 1).
		Render(text)
}

// SourceBadge creates a badge for a package source
func SourceBadge(source string) string {
	color, ok := SourceColors[source]
	if !ok {
		color = ColorMuted
	}
	return Badge(source, color)
}
