package tui

import (
	"poxy/internal/config"
	"poxy/internal/history"
	"poxy/pkg/database"
	"poxy/pkg/manager"
)

// View represents different views in the TUI
type View int

const (
	ViewPackages View = iota
	ViewSearch
	ViewUpdates
	ViewHistory
	ViewSystem
	ViewDetails
	ViewHelp
)

// Tab represents a navigable tab
type Tab struct {
	Name string
	View View
}

// DefaultTabs returns the default tab configuration
func DefaultTabs() []Tab {
	return []Tab{
		{Name: "Packages", View: ViewPackages},
		{Name: "Search", View: ViewSearch},
		{Name: "Updates", View: ViewUpdates},
		{Name: "History", View: ViewHistory},
		{Name: "System", View: ViewSystem},
	}
}

// Model holds the application state
type Model struct {
	// Core state
	ready    bool
	quitting bool

	// Dimensions
	width  int
	height int

	// Navigation
	tabs       []Tab
	activeTab  int
	activeView View
	prevView   View

	// Data
	registry       *manager.Registry
	config         *config.Config
	historyStore   *history.Store
	searchIndex    *database.Index
	installedPkgs  []manager.Package
	searchResults  []manager.Package
	historyEntries []history.Entry
	selectedPkg    *manager.Package

	// UI state
	loading      bool
	loadingMsg   string
	errorMsg     string
	successMsg   string
	filterText   string
	searchQuery  string
	inputMode    bool
	inputPrompt  string
	inputValue   string
	inputHandler func(string)

	// Cursor positions for each view
	cursors map[View]int

	// Scroll offsets for each view
	scrolls map[View]int

	// Styles and keys
	styles *Styles
	keys   KeyMap

	// Confirmation dialog
	showConfirm   bool
	confirmTitle  string
	confirmAction func()
}

// NewModel creates a new TUI model
func NewModel(registry *manager.Registry, cfg *config.Config, historyStore *history.Store, searchIndex *database.Index) *Model {
	return &Model{
		tabs:         DefaultTabs(),
		activeTab:    0,
		activeView:   ViewPackages,
		registry:     registry,
		config:       cfg,
		historyStore: historyStore,
		searchIndex:  searchIndex,
		cursors:      make(map[View]int),
		scrolls:      make(map[View]int),
		styles:       DefaultStyles(),
		keys:         DefaultKeyMap(),
	}
}

// SetSize sets the terminal size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// CurrentTab returns the current tab
func (m *Model) CurrentTab() Tab {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab]
	}
	return m.tabs[0]
}

// Cursor returns the cursor position for the current view
func (m *Model) Cursor() int {
	return m.cursors[m.activeView]
}

// SetCursor sets the cursor position for the current view
func (m *Model) SetCursor(pos int) {
	m.cursors[m.activeView] = pos
}

// Scroll returns the scroll offset for the current view
func (m *Model) Scroll() int {
	return m.scrolls[m.activeView]
}

// SetScroll sets the scroll offset for the current view
func (m *Model) SetScroll(offset int) {
	m.scrolls[m.activeView] = offset
}

// VisibleHeight returns the height available for list content
func (m *Model) VisibleHeight() int {
	// Account for header (2), tabs (1), footer (2), padding (2)
	return m.height - 7
}

// ListItems returns the items for the current view
func (m *Model) ListItems() []manager.Package {
	switch m.activeView {
	case ViewPackages:
		return m.filterPackages(m.installedPkgs)
	case ViewSearch:
		return m.searchResults
	case ViewUpdates:
		// TODO: Filter for upgradable packages
		return nil
	default:
		return nil
	}
}

// filterPackages filters packages by the current filter text
func (m *Model) filterPackages(pkgs []manager.Package) []manager.Package {
	if m.filterText == "" {
		return pkgs
	}

	var filtered []manager.Package
	for _, pkg := range pkgs {
		if containsIgnoreCase(pkg.Name, m.filterText) ||
			containsIgnoreCase(pkg.Description, m.filterText) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	sLower := toLowerCase(s)
	substrLower := toLowerCase(substr)
	return contains(sLower, substrLower)
}

func toLowerCase(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SelectedPackage returns the currently selected package
func (m *Model) SelectedPackage() *manager.Package {
	items := m.ListItems()
	cursor := m.Cursor()
	if cursor >= 0 && cursor < len(items) {
		return &items[cursor]
	}
	return nil
}

// MoveCursor moves the cursor by delta, clamping to valid range
func (m *Model) MoveCursor(delta int) {
	items := m.ListItems()
	if len(items) == 0 {
		return
	}

	newPos := m.Cursor() + delta
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= len(items) {
		newPos = len(items) - 1
	}
	m.SetCursor(newPos)

	// Adjust scroll to keep cursor visible
	visibleHeight := m.VisibleHeight()
	scroll := m.Scroll()

	if newPos < scroll {
		m.SetScroll(newPos)
	} else if newPos >= scroll+visibleHeight {
		m.SetScroll(newPos - visibleHeight + 1)
	}
}

// GoToTop moves cursor to the top
func (m *Model) GoToTop() {
	m.SetCursor(0)
	m.SetScroll(0)
}

// GoToBottom moves cursor to the bottom
func (m *Model) GoToBottom() {
	items := m.ListItems()
	if len(items) == 0 {
		return
	}
	m.SetCursor(len(items) - 1)

	visibleHeight := m.VisibleHeight()
	if len(items) > visibleHeight {
		m.SetScroll(len(items) - visibleHeight)
	}
}

// NextTab switches to the next tab
func (m *Model) NextTab() {
	m.activeTab = (m.activeTab + 1) % len(m.tabs)
	m.activeView = m.tabs[m.activeTab].View
}

// PrevTab switches to the previous tab
func (m *Model) PrevTab() {
	m.activeTab--
	if m.activeTab < 0 {
		m.activeTab = len(m.tabs) - 1
	}
	m.activeView = m.tabs[m.activeTab].View
}

// SetTab switches to a specific tab by index
func (m *Model) SetTab(index int) {
	if index >= 0 && index < len(m.tabs) {
		m.activeTab = index
		m.activeView = m.tabs[m.activeTab].View
	}
}

// ShowDetails shows the details view for the selected package
func (m *Model) ShowDetails() {
	if pkg := m.SelectedPackage(); pkg != nil {
		m.selectedPkg = pkg
		m.prevView = m.activeView
		m.activeView = ViewDetails
	}
}

// GoBack returns to the previous view
func (m *Model) GoBack() {
	if m.activeView == ViewDetails || m.activeView == ViewHelp {
		m.activeView = m.prevView
	}
}

// SetLoading sets the loading state
func (m *Model) SetLoading(loading bool, msg string) {
	m.loading = loading
	m.loadingMsg = msg
}

// SetError sets an error message
func (m *Model) SetError(msg string) {
	m.errorMsg = msg
	m.successMsg = ""
}

// SetSuccess sets a success message
func (m *Model) SetSuccess(msg string) {
	m.successMsg = msg
	m.errorMsg = ""
}

// ClearMessages clears all messages
func (m *Model) ClearMessages() {
	m.errorMsg = ""
	m.successMsg = ""
}

// StartInput starts input mode
func (m *Model) StartInput(prompt string, handler func(string)) {
	m.inputMode = true
	m.inputPrompt = prompt
	m.inputValue = ""
	m.inputHandler = handler
}

// FinishInput finishes input mode and calls the handler
func (m *Model) FinishInput() {
	if m.inputHandler != nil {
		m.inputHandler(m.inputValue)
	}
	m.inputMode = false
	m.inputPrompt = ""
	m.inputValue = ""
	m.inputHandler = nil
}

// CancelInput cancels input mode
func (m *Model) CancelInput() {
	m.inputMode = false
	m.inputPrompt = ""
	m.inputValue = ""
	m.inputHandler = nil
}

// ShowConfirm shows a confirmation dialog
func (m *Model) ShowConfirm(title string, action func()) {
	m.showConfirm = true
	m.confirmTitle = title
	m.confirmAction = action
}

// ConfirmYes executes the confirmation action
func (m *Model) ConfirmYes() {
	if m.confirmAction != nil {
		m.confirmAction()
	}
	m.showConfirm = false
	m.confirmTitle = ""
	m.confirmAction = nil
}

// ConfirmNo cancels the confirmation
func (m *Model) ConfirmNo() {
	m.showConfirm = false
	m.confirmTitle = ""
	m.confirmAction = nil
}
