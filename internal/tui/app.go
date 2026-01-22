package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"poxy/internal/config"
	"poxy/internal/history"
	"poxy/pkg/database"
	"poxy/pkg/manager"
)

// Messages for async operations
type (
	packagesLoadedMsg struct {
		packages []manager.Package
		err      error
	}

	searchResultsMsg struct {
		results []manager.Package
		err     error
	}

	historyLoadedMsg struct {
		entries []history.Entry
		err     error
	}

	operationCompleteMsg struct {
		success bool
		message string
		err     error
	}
)

// App wraps the Model with bubbletea components
type App struct {
	*Model
	spinner   spinner.Model
	textInput textinput.Model
}

// NewApp creates a new TUI application
func NewApp(registry *manager.Registry, cfg *config.Config, historyStore *history.Store, searchIndex *database.Index) *App {
	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 100
	ti.Width = 40

	return &App{
		Model:     NewModel(registry, cfg, historyStore, searchIndex),
		spinner:   sp,
		textInput: ti,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.loadPackages(),
		a.loadHistory(),
	)
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.SetSize(msg.Width, msg.Height)
		a.ready = true

	case tea.KeyMsg:
		// Handle confirmation dialog first
		if a.showConfirm {
			switch msg.String() {
			case "y", "Y", "enter":
				a.ConfirmYes()
			case "n", "N", "esc", "q":
				a.ConfirmNo()
			}
			return a, nil
		}

		// Handle input mode
		if a.inputMode {
			switch msg.String() {
			case "enter":
				a.FinishInput()
				return a, nil
			case "esc":
				a.CancelInput()
				return a, nil
			default:
				var cmd tea.Cmd
				a.textInput, cmd = a.textInput.Update(msg)
				a.inputValue = a.textInput.Value()
				cmds = append(cmds, cmd)
				return a, tea.Batch(cmds...)
			}
		}

		// Global keybindings
		switch {
		case key.Matches(msg, a.keys.Quit):
			a.quitting = true
			return a, tea.Quit

		case key.Matches(msg, a.keys.Help):
			if a.activeView == ViewHelp {
				a.GoBack()
			} else {
				a.prevView = a.activeView
				a.activeView = ViewHelp
			}

		case key.Matches(msg, a.keys.Tab1):
			a.SetTab(0)
		case key.Matches(msg, a.keys.Tab2):
			a.SetTab(1)
			if a.activeView == ViewSearch && a.searchQuery == "" {
				a.startSearch()
			}
		case key.Matches(msg, a.keys.Tab3):
			a.SetTab(2)
		case key.Matches(msg, a.keys.Tab4):
			a.SetTab(3)
		case key.Matches(msg, a.keys.Tab5):
			a.SetTab(4)

		case key.Matches(msg, a.keys.Left):
			a.PrevTab()
		case key.Matches(msg, a.keys.Right):
			a.NextTab()

		case key.Matches(msg, a.keys.Back):
			a.GoBack()
		case key.Matches(msg, a.keys.Cancel):
			a.GoBack()
			a.ClearMessages()

		// Navigation
		case key.Matches(msg, a.keys.Up), key.Matches(msg, a.keys.VimUp):
			a.MoveCursor(-1)
		case key.Matches(msg, a.keys.Down), key.Matches(msg, a.keys.VimDown):
			a.MoveCursor(1)
		case key.Matches(msg, a.keys.PageUp):
			a.MoveCursor(-a.VisibleHeight())
		case key.Matches(msg, a.keys.PageDown):
			a.MoveCursor(a.VisibleHeight())
		case key.Matches(msg, a.keys.Home), key.Matches(msg, a.keys.VimTop):
			a.GoToTop()
		case key.Matches(msg, a.keys.End), key.Matches(msg, a.keys.VimBot):
			a.GoToBottom()

		// Actions
		case key.Matches(msg, a.keys.Enter):
			if a.activeView == ViewPackages || a.activeView == ViewSearch {
				a.ShowDetails()
			}

		case key.Matches(msg, a.keys.Search):
			a.startSearch()

		case key.Matches(msg, a.keys.Filter):
			a.startFilter()

		case key.Matches(msg, a.keys.Install):
			if pkg := a.SelectedPackage(); pkg != nil && !pkg.Installed {
				a.ShowConfirm(fmt.Sprintf("Install %s?", pkg.Name), func() {
					cmds = append(cmds, a.installPackage(pkg.Name, pkg.Source))
				})
			}

		case key.Matches(msg, a.keys.Uninstall):
			if pkg := a.SelectedPackage(); pkg != nil && pkg.Installed {
				a.ShowConfirm(fmt.Sprintf("Remove %s?", pkg.Name), func() {
					cmds = append(cmds, a.uninstallPackage(pkg.Name, pkg.Source))
				})
			}

		case key.Matches(msg, a.keys.Update):
			a.ShowConfirm("Update package databases?", func() {
				cmds = append(cmds, a.updateDatabases())
			})
		}

	case packagesLoadedMsg:
		a.SetLoading(false, "")
		if msg.err != nil {
			a.SetError(msg.err.Error())
		} else {
			a.installedPkgs = msg.packages
		}

	case searchResultsMsg:
		a.SetLoading(false, "")
		if msg.err != nil {
			a.SetError(msg.err.Error())
		} else {
			a.searchResults = msg.results
			if len(msg.results) == 0 {
				a.SetError("No packages found")
			}
		}

	case historyLoadedMsg:
		if msg.err == nil {
			a.historyEntries = msg.entries
		}

	case operationCompleteMsg:
		a.SetLoading(false, "")
		if msg.err != nil {
			a.SetError(msg.err.Error())
		} else if msg.success {
			a.SetSuccess(msg.message)
			// Reload packages after successful operation
			cmds = append(cmds, a.loadPackages())
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a *App) View() string {
	if !a.ready {
		return "Loading..."
	}

	if a.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(a.renderHeader())
	b.WriteString("\n")

	// Tabs
	b.WriteString(a.renderTabs())
	b.WriteString("\n")

	// Content
	b.WriteString(a.renderContent())

	// Footer
	b.WriteString(a.renderFooter())

	// Overlay: Confirmation dialog
	if a.showConfirm {
		return a.renderWithDialog(b.String())
	}

	return b.String()
}

// renderHeader renders the header bar
func (a *App) renderHeader() string {
	title := a.styles.Header.Render(" Poxy - Universal Package Manager ")

	// Right side: loading indicator or status
	var right string
	if a.loading {
		right = a.spinner.View() + " " + a.loadingMsg
	} else if a.errorMsg != "" {
		right = a.styles.Error.Render(a.errorMsg)
	} else if a.successMsg != "" {
		right = a.styles.Success.Render(a.successMsg)
	}

	// Pad to full width
	padding := a.width - lipgloss.Width(title) - lipgloss.Width(right) - 2
	if padding < 0 {
		padding = 0
	}

	return title + strings.Repeat(" ", padding) + right
}

// renderTabs renders the tab bar
func (a *App) renderTabs() string {
	var tabs []string
	for i, tab := range a.tabs {
		style := a.styles.TabInactive
		if i == a.activeTab {
			style = a.styles.TabActive
		}
		tabs = append(tabs, style.Render(fmt.Sprintf("[%d] %s", i+1, tab.Name)))
	}

	tabBar := strings.Join(tabs, " ")
	return lipgloss.NewStyle().
		Width(a.width).
		Background(ColorBgAlt).
		Padding(0, 1).
		Render(tabBar)
}

// renderContent renders the main content area
func (a *App) renderContent() string {
	height := a.height - 5 // Account for header, tabs, footer

	var content string
	switch a.activeView {
	case ViewPackages:
		content = a.renderPackageList(a.installedPkgs, "Installed Packages")
	case ViewSearch:
		content = a.renderSearchView()
	case ViewUpdates:
		content = a.renderUpdatesView()
	case ViewHistory:
		content = a.renderHistoryView()
	case ViewSystem:
		content = a.renderSystemView()
	case ViewDetails:
		content = a.renderDetailsView()
	case ViewHelp:
		content = a.renderHelpView()
	}

	return lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		Render(content)
}

// renderPackageList renders a list of packages
func (a *App) renderPackageList(packages []manager.Package, title string) string {
	var b strings.Builder

	// Apply filter
	filtered := a.filterPackages(packages)

	// Title with count
	titleStr := fmt.Sprintf("%s (%d)", title, len(filtered))
	if a.filterText != "" {
		titleStr += fmt.Sprintf(" - Filter: %s", a.filterText)
	}
	b.WriteString(a.styles.Title.Render(titleStr))
	b.WriteString("\n\n")

	if len(filtered) == 0 {
		b.WriteString(a.styles.Description.Render("No packages found"))
		return b.String()
	}

	// Calculate visible range
	visibleHeight := a.VisibleHeight()
	scroll := a.Scroll()
	cursor := a.Cursor()

	start := scroll
	end := scroll + visibleHeight
	if end > len(filtered) {
		end = len(filtered)
	}

	// Render visible items
	for i := start; i < end; i++ {
		pkg := filtered[i]
		isSelected := i == cursor

		line := a.renderPackageLine(pkg, isSelected)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(filtered) > visibleHeight {
		scrollPct := float64(scroll) / float64(len(filtered)-visibleHeight) * 100
		b.WriteString(a.styles.Description.Render(fmt.Sprintf("\n  %.0f%% (%d/%d)", scrollPct, cursor+1, len(filtered))))
	}

	return b.String()
}

// renderPackageLine renders a single package line
func (a *App) renderPackageLine(pkg manager.Package, selected bool) string {
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = a.styles.ListItemSelected.Render("> ")
	}

	// Package name
	name := a.styles.PackageName.Render(pkg.Name)
	if !selected {
		name = lipgloss.NewStyle().Foreground(ColorText).Render(pkg.Name)
	}

	// Version
	version := a.styles.PackageVersion.Render(pkg.Version)

	// Source badge
	source := SourceBadge(pkg.Source)

	// Description (truncated)
	maxDescWidth := a.width - lipgloss.Width(cursor) - lipgloss.Width(name) - lipgloss.Width(version) - lipgloss.Width(source) - 10
	desc := pkg.Description
	if len(desc) > maxDescWidth && maxDescWidth > 3 {
		desc = desc[:maxDescWidth-3] + "..."
	}
	descStyle := a.styles.PackageDesc.Render(desc)

	return fmt.Sprintf("%s%-25s %s %s %s", cursor, name, version, source, descStyle)
}

// renderSearchView renders the search view
func (a *App) renderSearchView() string {
	var b strings.Builder

	// Search input
	if a.inputMode && a.inputPrompt == "Search: " {
		b.WriteString(a.styles.InputPrompt.Render("Search: "))
		b.WriteString(a.textInput.View())
		b.WriteString("\n\n")
	} else if a.searchQuery != "" {
		b.WriteString(a.styles.Title.Render(fmt.Sprintf("Search results for '%s'", a.searchQuery)))
		b.WriteString("\n\n")
	} else {
		b.WriteString(a.styles.Title.Render("Search Packages"))
		b.WriteString("\n")
		b.WriteString(a.styles.Description.Render("Press / to search"))
		b.WriteString("\n\n")
	}

	if len(a.searchResults) > 0 {
		b.WriteString(a.renderPackageListContent(a.searchResults))
	} else if a.searchQuery != "" && !a.loading {
		b.WriteString(a.styles.Description.Render("No results found"))
	}

	return b.String()
}

// renderPackageListContent renders just the package list content
func (a *App) renderPackageListContent(packages []manager.Package) string {
	var b strings.Builder

	visibleHeight := a.VisibleHeight() - 4 // Account for search header
	scroll := a.Scroll()
	cursor := a.Cursor()

	start := scroll
	end := scroll + visibleHeight
	if end > len(packages) {
		end = len(packages)
	}

	for i := start; i < end; i++ {
		pkg := packages[i]
		isSelected := i == cursor
		b.WriteString(a.renderPackageLine(pkg, isSelected))
		b.WriteString("\n")
	}

	return b.String()
}

// renderUpdatesView renders the updates view
func (a *App) renderUpdatesView() string {
	var b strings.Builder

	b.WriteString(a.styles.Title.Render("Available Updates"))
	b.WriteString("\n\n")
	b.WriteString(a.styles.Description.Render("Press 'u' to check for updates"))
	b.WriteString("\n\n")

	// TODO: Implement update checking
	b.WriteString(a.styles.Info.Render("Update checking not yet implemented"))

	return b.String()
}

// renderHistoryView renders the history view
func (a *App) renderHistoryView() string {
	var b strings.Builder

	b.WriteString(a.styles.Title.Render("Operation History"))
	b.WriteString("\n\n")

	if len(a.historyEntries) == 0 {
		b.WriteString(a.styles.Description.Render("No history entries"))
		return b.String()
	}

	for i, entry := range a.historyEntries {
		if i >= a.VisibleHeight() {
			break
		}

		// Format: [time] operation packages (status)
		status := a.styles.Success.Render("OK")
		if !entry.Success {
			status = a.styles.Error.Render("FAILED")
		}

		timestamp := entry.Timestamp.Format("2006-01-02 15:04")
		op := string(entry.Operation)
		pkgs := strings.Join(entry.Packages, ", ")
		if len(pkgs) > 40 {
			pkgs = pkgs[:37] + "..."
		}

		line := fmt.Sprintf("  %s  %-10s  %-40s  %s", timestamp, op, pkgs, status)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderSystemView renders the system information view
func (a *App) renderSystemView() string {
	var b strings.Builder

	b.WriteString(a.styles.Title.Render("System Information"))
	b.WriteString("\n\n")

	sysInfo := a.registry.SystemInfo()
	if sysInfo == nil {
		b.WriteString(a.styles.Error.Render("System information not available"))
		return b.String()
	}

	// System info
	b.WriteString(a.styles.Subtitle.Render("Operating System"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  OS:       %s\n", sysInfo.OS))
	b.WriteString(fmt.Sprintf("  Distro:   %s\n", sysInfo.PrettyName))
	b.WriteString(fmt.Sprintf("  Version:  %s\n", sysInfo.VersionID))
	b.WriteString(fmt.Sprintf("  Arch:     %s\n", sysInfo.Arch))
	b.WriteString("\n")

	// Native manager
	b.WriteString(a.styles.Subtitle.Render("Package Managers"))
	b.WriteString("\n")
	if native := a.registry.Native(); native != nil {
		b.WriteString(fmt.Sprintf("  Native:   %s\n", native.DisplayName()))
	}

	// Available managers
	b.WriteString("\n")
	b.WriteString(a.styles.Subtitle.Render("Available Sources"))
	b.WriteString("\n")
	for _, mgr := range a.registry.Available() {
		status := a.styles.Success.Render("OK")
		sudo := ""
		if mgr.NeedsSudo() {
			sudo = " (sudo)"
		}
		b.WriteString(fmt.Sprintf("  %-12s %s%s\n", mgr.Name(), status, sudo))
	}

	return b.String()
}

// renderDetailsView renders package details
func (a *App) renderDetailsView() string {
	var b strings.Builder

	if a.selectedPkg == nil {
		b.WriteString(a.styles.Error.Render("No package selected"))
		return b.String()
	}

	pkg := a.selectedPkg

	// Header
	b.WriteString(a.styles.Title.Render(pkg.Name))
	b.WriteString(" ")
	b.WriteString(SourceBadge(pkg.Source))
	b.WriteString("\n\n")

	// Version
	b.WriteString(a.styles.Subtitle.Render("Version: "))
	b.WriteString(a.styles.PackageVersion.Render(pkg.Version))
	b.WriteString("\n\n")

	// Description
	b.WriteString(a.styles.Subtitle.Render("Description"))
	b.WriteString("\n")
	b.WriteString(a.styles.Description.Render(pkg.Description))
	b.WriteString("\n\n")

	// Status
	b.WriteString(a.styles.Subtitle.Render("Status: "))
	if pkg.Installed {
		b.WriteString(a.styles.Success.Render("Installed"))
	} else {
		b.WriteString(a.styles.Info.Render("Not installed"))
	}
	b.WriteString("\n\n")

	// Actions
	b.WriteString(a.styles.Subtitle.Render("Actions"))
	b.WriteString("\n")
	if pkg.Installed {
		b.WriteString("  [r] Remove package\n")
	} else {
		b.WriteString("  [i] Install package\n")
	}
	b.WriteString("  [b] Back\n")

	return b.String()
}

// renderHelpView renders the help view
func (a *App) renderHelpView() string {
	var b strings.Builder

	b.WriteString(a.styles.Title.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			title: "Navigation",
			keys: []struct{ key, desc string }{
				{"j/k or Up/Down", "Move cursor"},
				{"g/G", "Go to top/bottom"},
				{"PgUp/PgDn", "Page up/down"},
				{"1-5", "Switch tabs"},
				{"Left/Right", "Previous/next tab"},
			},
		},
		{
			title: "Actions",
			keys: []struct{ key, desc string }{
				{"Enter", "View details"},
				{"/", "Search packages"},
				{"f", "Filter list"},
				{"i", "Install package"},
				{"r", "Remove package"},
				{"u", "Update databases"},
			},
		},
		{
			title: "General",
			keys: []struct{ key, desc string }{
				{"?", "Toggle help"},
				{"Esc/b", "Go back"},
				{"q", "Quit"},
			},
		},
	}

	for _, section := range sections {
		b.WriteString(a.styles.Subtitle.Render(section.title))
		b.WriteString("\n")
		for _, k := range section.keys {
			b.WriteString(fmt.Sprintf("  %s%-20s%s %s\n",
				a.styles.HelpKey.Render(""),
				a.styles.HelpKey.Render(k.key),
				a.styles.HelpSep.String(),
				a.styles.HelpDesc.Render(k.desc)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderFooter renders the footer bar
func (a *App) renderFooter() string {
	var hints []string

	switch a.activeView {
	case ViewPackages, ViewSearch:
		hints = []string{"i:install", "r:remove", "/:search", "Enter:details"}
	case ViewDetails:
		if a.selectedPkg != nil && a.selectedPkg.Installed {
			hints = []string{"r:remove", "b:back"}
		} else {
			hints = []string{"i:install", "b:back"}
		}
	case ViewHistory:
		hints = []string{"Enter:details", "b:back"}
	default:
		hints = []string{"?:help", "q:quit"}
	}

	hints = append(hints, "?:help", "q:quit")

	footer := strings.Join(hints, "  ")
	return lipgloss.NewStyle().
		Width(a.width).
		Background(ColorBgAlt).
		Foreground(ColorMuted).
		Padding(0, 1).
		Render(footer)
}

// renderWithDialog renders content with a dialog overlay
func (a *App) renderWithDialog(_ string) string {
	dialog := a.styles.Dialog.Render(
		a.styles.DialogTitle.Render(a.confirmTitle) + "\n\n" +
			a.styles.DialogButton.Render("[Y]es") + " " +
			lipgloss.NewStyle().Foreground(ColorMuted).Render("[N]o"),
	)

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(ColorBg))
}

// startSearch initiates search input
func (a *App) startSearch() {
	a.textInput.SetValue("")
	a.textInput.Focus()
	a.StartInput("Search: ", func(query string) {
		if query != "" {
			a.searchQuery = query
			a.SetLoading(true, "Searching...")
			// Search will be triggered via command
		}
	})
}

// startFilter initiates filter input
func (a *App) startFilter() {
	a.textInput.SetValue(a.filterText)
	a.textInput.Focus()
	a.StartInput("Filter: ", func(filter string) {
		a.filterText = filter
		a.SetCursor(0)
		a.SetScroll(0)
	})
}

// Async commands

func (a *App) loadPackages() tea.Cmd {
	return func() tea.Msg {
		a.SetLoading(true, "Loading packages...")

		ctx := context.Background()
		var allPkgs []manager.Package

		for _, mgr := range a.registry.Available() {
			pkgs, err := mgr.ListInstalled(ctx, manager.ListOpts{})
			if err != nil {
				continue
			}
			allPkgs = append(allPkgs, pkgs...)
		}

		return packagesLoadedMsg{packages: allPkgs}
	}
}

func (a *App) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if a.historyStore == nil {
			return historyLoadedMsg{}
		}

		entries, err := a.historyStore.List(50)
		return historyLoadedMsg{entries: entries, err: err}
	}
}

func (a *App) installPackage(name, source string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		mgr, ok := a.registry.Get(source)
		if !ok {
			return operationCompleteMsg{err: fmt.Errorf("unknown source: %s", source)}
		}

		err := mgr.Install(ctx, []string{name}, manager.InstallOpts{AutoConfirm: true})
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{success: true, message: fmt.Sprintf("Installed %s", name)}
	}
}

func (a *App) uninstallPackage(name, source string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		mgr, ok := a.registry.Get(source)
		if !ok {
			return operationCompleteMsg{err: fmt.Errorf("unknown source: %s", source)}
		}

		err := mgr.Uninstall(ctx, []string{name}, manager.UninstallOpts{AutoConfirm: true})
		if err != nil {
			return operationCompleteMsg{err: err}
		}

		return operationCompleteMsg{success: true, message: fmt.Sprintf("Removed %s", name)}
	}
}

func (a *App) updateDatabases() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		if native := a.registry.Native(); native != nil {
			if err := native.Update(ctx); err != nil {
				return operationCompleteMsg{err: err}
			}
		}

		return operationCompleteMsg{success: true, message: "Package databases updated"}
	}
}

// Run starts the TUI application
func Run(registry *manager.Registry, cfg *config.Config, historyStore *history.Store, searchIndex *database.Index) error {
	app := NewApp(registry, cfg, historyStore, searchIndex)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
