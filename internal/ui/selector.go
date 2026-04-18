package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectorItem is a row in an interactive selector.
type SelectorItem struct {
	Title string
	Desc  string
}

var (
	selectorTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	selectorCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	selectorActiveTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selectorActiveDesc  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	selectorFaintStyle  = lipgloss.NewStyle().Faint(true)
	selectorHelpStyle   = lipgloss.NewStyle().Faint(true).MarginTop(1)
)

type selectorModel struct {
	title     string
	items     []SelectorItem
	cursor    int
	chosen    int
	cancelled bool
	quitting  bool
}

func (m selectorModel) Init() tea.Cmd { return nil }

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q", "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.chosen = m.cursor
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.items) - 1
		}
	}
	return m, nil
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	if m.title != "" {
		b.WriteString(selectorTitleStyle.Render(m.title))
		b.WriteString("\n\n")
	}

	for i, item := range m.items {
		var line string
		if i == m.cursor {
			line = selectorCursorStyle.Render("▸ ") + selectorActiveTitle.Render(item.Title)
			if item.Desc != "" {
				line += " " + selectorActiveDesc.Render(item.Desc)
			}
		} else {
			line = "  " + item.Title
			if item.Desc != "" {
				line += " " + selectorFaintStyle.Render(item.Desc)
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(selectorHelpStyle.Render("↑/k up • ↓/j down • enter select • esc/q quit"))
	return b.String()
}

// RunSelector shows an interactive list of items and returns the selected index.
// Returns -1 if the user cancelled with esc/q/ctrl+c.
func RunSelector(title string, items []SelectorItem) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("no items to select from")
	}

	m := selectorModel{title: title, items: items, chosen: -1}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return -1, err
	}

	fm, ok := final.(selectorModel)
	if !ok {
		return -1, fmt.Errorf("unexpected selector state")
	}
	if fm.cancelled {
		return -1, nil
	}
	return fm.chosen, nil
}
