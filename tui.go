package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type state int

const (
	stateLoading state = iota
	stateComposing
	stateCommitting
	stateDone
	stateError
)

type screen int

const (
	screenCompose screen = iota
	screenJournalPicker
	screenLogs
	screenThemePicker
)

// Messages
type geoMsg struct {
	loc Location
	err error
}

type commitDoneMsg struct {
	err error
}

type syncStatusMsg struct {
	journal string
	status  SyncStatus
	err     error
}

// Styles
var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	subtitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	hintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("84")).Bold(true)
	selectedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	accentStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	sectionLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
)

func renderPanel(lines ...string) string {
	return panelStyle.Render(strings.Join(lines, "\n"))
}

func renderPanelWithWidth(totalWidth int, lines ...string) string {
	if totalWidth <= 0 {
		return renderPanel(lines...)
	}
	inner := totalWidth - 6 // indent(2) + border/padding(4)
	if inner < 10 {
		inner = 10
	}
	return panelStyle.Width(inner).Render(strings.Join(lines, "\n"))
}

func indentBlock(s, prefix string) string {
	if s == "" || prefix == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderHead(title, subtitle string) string {
	if strings.TrimSpace(subtitle) == "" {
		return "  " + titleStyle.Render(title)
	}
	return "  " + titleStyle.Render(title) + " " + hintStyle.Render("· "+subtitle)
}

type model struct {
	state    state
	screen   screen
	spinner  spinner.Model
	textarea textarea.Model
	locating bool
	location Location
	now      time.Time
	repoPath string
	err      error

	cfg          *Config
	journalNames []string
	journalIndex int
	themeNames   []string
	themeIndex   int
	themeFrom    screen
	themeErr     error

	syncFor     string
	syncStatus  *SyncStatus
	syncErr     error
	syncLoading bool

	list listModel

	windowHeight int
	windowWidth  int
}

func newModel(cfg *Config) model {
	cfg.migrateLegacy()
	activeName, repoPath, _ := cfg.activeJournal()
	names := cfg.journalNames()
	activeIdx := 0
	for i, name := range names {
		if name == activeName {
			activeIdx = i
			break
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accentStyle

	ta := textarea.New()
	ta.Placeholder = ""
	ta.ShowLineNumbers = false
	ta.SetWidth(60)
	ta.SetHeight(8)
	ta.Focus()

	// Remove default textarea borders/prompts for a clean look
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()

	return model{
		state:        stateComposing,
		screen:       screenCompose,
		spinner:      sp,
		textarea:     ta,
		locating:     true,
		now:          time.Now(),
		repoPath:     repoPath,
		cfg:          cfg,
		journalNames: names,
		journalIndex: activeIdx,
		syncFor:      activeName,
		syncLoading:  true,
	}
}

func (m model) Init() tea.Cmd {
	activeName := m.syncFor
	if activeName == "" {
		activeName, _, _ = m.cfg.activeJournal()
	}
	return tea.Batch(
		m.spinner.Tick,
		fetchGeoCmd(),
		syncStatusCmd(activeName, m.repoPath),
	)
}

func fetchGeoCmd() tea.Cmd {
	return func() tea.Msg {
		loc, err := fetchLocation()
		return geoMsg{loc: loc, err: err}
	}
}

func commitCmd(repoPath, entry string, loc Location) tea.Cmd {
	return func() tea.Msg {
		// If async geolocation hasn't resolved yet, make one best-effort lookup
		// so entries still capture location when available.
		if loc.String() == "" {
			if fetched, err := fetchLocation(); err == nil {
				loc = fetched
			}
		}
		err := commitEntry(repoPath, entry, loc)
		return commitDoneMsg{err: err}
	}
}

func syncStatusCmd(journalName, repoPath string) tea.Cmd {
	return func() tea.Msg {
		status, err := getSyncStatus(repoPath)
		return syncStatusMsg{
			journal: journalName,
			status:  status,
			err:     err,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.screen == screenThemePicker {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc":
				m.screen = m.themeFrom
				return m, nil
			case "up", "k":
				if m.themeIndex > 0 {
					m.themeIndex--
				}
				return m, nil
			case "down", "j":
				if m.themeIndex < len(m.themeNames)-1 {
					m.themeIndex++
				}
				return m, nil
			case "enter":
				if err := m.setTheme(m.selectedThemeName()); err != nil {
					m.themeErr = err
					return m, nil
				}
				m.screen = m.themeFrom
				return m, nil
			}
		}
		return m, nil
	}

	if m.screen == screenLogs {
		if winMsg, ok := msg.(tea.WindowSizeMsg); ok {
			m.windowHeight = winMsg.Height
			m.windowWidth = winMsg.Width
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "q":
				m.screen = screenCompose
				return m, nil
			case "esc":
				if m.list.state != listDetail {
					m.screen = screenCompose
					return m, nil
				}
			}
		}
		next, cmd := m.list.Update(msg)
		if lm, ok := next.(listModel); ok {
			m.list = lm
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		taWidth := m.windowWidth - 10
		if taWidth < 20 {
			taWidth = 20
		}
		m.textarea.SetWidth(taWidth)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.screen == screenJournalPicker {
				m.screen = screenCompose
				return m, nil
			}
			if m.state == stateComposing {
				return m, tea.Quit
			}
			if m.state == stateError {
				return m, tea.Quit
			}
		case tea.KeyCtrlS:
			if m.state == stateComposing {
				entry := strings.TrimSpace(m.textarea.Value())
				if entry == "" {
					return m, nil
				}
				m.state = stateCommitting
				return m, tea.Batch(
					m.spinner.Tick,
					commitCmd(m.repoPath, entry, m.location),
				)
			}
		}

		if m.state == stateComposing && m.screen == screenCompose {
			switch msg.String() {
			case "ctrl+o":
				m.screen = screenJournalPicker
				return m, nil
			case "ctrl+t":
				return m.openThemePicker(screenCompose)
			case "ctrl+l":
				return m.openLogsFor(m.activeJournalName())
			case "ctrl+r":
				return m.refreshSyncFor(m.activeJournalName())
			}
		}

		if m.state == stateComposing && m.screen == screenJournalPicker {
			switch msg.String() {
			case "up", "k":
				if m.journalIndex > 0 {
					m.journalIndex--
				}
				return m, nil
			case "down", "j":
				if m.journalIndex < len(m.journalNames)-1 {
					m.journalIndex++
				}
				return m, nil
			case "enter":
				if err := m.switchActiveJournal(m.selectedJournalName()); err != nil {
					m.syncErr = err
					return m, nil
				}
				m.screen = screenCompose
				return m.refreshSyncFor(m.activeJournalName())
			case "l":
				return m.openLogsFor(m.selectedJournalName())
			case "s":
				return m.refreshSyncFor(m.selectedJournalName())
			case "t":
				return m.openThemePicker(screenJournalPicker)
			}
		}

	case geoMsg:
		m.locating = false
		if msg.err == nil {
			m.location = msg.loc
		}
		return m, textarea.Blink

	case syncStatusMsg:
		m.syncFor = msg.journal
		m.syncLoading = false
		if msg.err != nil {
			m.syncErr = msg.err
			m.syncStatus = nil
		} else {
			m.syncErr = nil
			status := msg.status
			m.syncStatus = &status
		}
		return m, nil

	case commitDoneMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
		} else {
			m.state = stateDone
		}
		return m, tea.Quit

	case spinner.TickMsg:
		if m.locating || m.state == stateCommitting || m.syncLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if m.state == stateComposing && m.screen == screenCompose {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.screen == screenLogs {
		return m.list.View()
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(renderHead("journal", "write") + "\n")

	switch m.state {
	case stateComposing:
		if m.screen == screenThemePicker {
			var body strings.Builder
			body.WriteString(sectionLabelStyle.Render("Themes") + "\n")
			for i, name := range m.themeNames {
				line := name
				if name == m.cfg.Theme {
					line += " (active)"
				}
				if i == m.themeIndex {
					body.WriteString(accentStyle.Render("▶") + " " + selectedStyle.Render(line) + "\n")
				} else {
					body.WriteString("  " + hintStyle.Render(line) + "\n")
				}
			}
			if m.themeErr != nil {
				body.WriteString("\n" + errorStyle.Render(m.themeErr.Error()))
			}
			b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, body.String()), "  ") + "\n")
			b.WriteString("  " + hintStyle.Render("↑↓ select  ·  enter apply  ·  esc back") + "\n")
		} else if m.screen == screenJournalPicker {
			var body strings.Builder
			body.WriteString(sectionLabelStyle.Render("Journals") + "\n")
			for i, name := range m.journalNames {
				line := name
				if name == m.activeJournalName() {
					line += " (active)"
				}
				if i == m.journalIndex {
					body.WriteString(accentStyle.Render("▶") + " " + selectedStyle.Render(line) + "\n")
				} else {
					body.WriteString("  " + hintStyle.Render(line) + "\n")
				}
			}
			body.WriteString("\n" + subtitleStyle.Render(m.syncSummary()))
			b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, body.String()), "  ") + "\n")
			b.WriteString("  " + hintStyle.Render("↑↓ select  ·  enter activate  ·  l logs  ·  s sync status  ·  t themes  ·  esc back") + "\n")
		} else {
			var body strings.Builder
			body.WriteString(sectionLabelStyle.Render(formatComposeHeader(m.now, m.location, m.locating, m.spinner)) + "\n")
			body.WriteString(subtitleStyle.Render("active: "+m.activeJournalName()) + "\n")
			body.WriteString(subtitleStyle.Render(m.syncSummary()) + "\n\n")
			lines := strings.Split(m.textarea.View(), "\n")
			for _, line := range lines {
				body.WriteString(line + "\n")
			}
			b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, strings.TrimRight(body.String(), "\n")), "  ") + "\n")
			b.WriteString("  " + hintStyle.Render("ctrl+s commit  ·  ctrl+o journals  ·  ctrl+t themes  ·  ctrl+l logs  ·  ctrl+r sync status  ·  esc quit") + "\n")
		}

	case stateCommitting:
		header := formatDate(m.now)
		if location := m.location.String(); location != "" {
			header += " · " + location
		}
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, sectionLabelStyle.Render(header), "", m.spinner.View()+" "+subtitleStyle.Render("committing...")), "  ") + "\n")

	case stateDone:
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, successStyle.Render("entry saved.")), "  ") + "\n")

	case stateError:
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, errorStyle.Render(fmt.Sprintf("error: %v", m.err))), "  ") + "\n")
		b.WriteString("  " + hintStyle.Render("press any key to exit") + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *model) activeJournalName() string {
	if m.cfg == nil {
		return ""
	}
	return m.cfg.ActiveJournal
}

func (m *model) selectedJournalName() string {
	if len(m.journalNames) == 0 {
		return ""
	}
	if m.journalIndex < 0 {
		m.journalIndex = 0
	}
	if m.journalIndex >= len(m.journalNames) {
		m.journalIndex = len(m.journalNames) - 1
	}
	return m.journalNames[m.journalIndex]
}

func (m *model) openLogsFor(name string) (tea.Model, tea.Cmd) {
	_, repoPath, err := m.cfg.journal(name)
	if err != nil {
		m.syncErr = err
		return m, nil
	}
	m.list = newListModel(repoPath)
	m.list.windowHeight = m.windowHeight
	m.list.windowWidth = m.windowWidth
	m.screen = screenLogs
	return m, m.list.Init()
}

func (m *model) openThemePicker(from screen) (tea.Model, tea.Cmd) {
	themes, err := loadAllThemes()
	if err != nil {
		m.themeErr = err
		return m, nil
	}
	m.themeNames = themeNames(themes)
	m.themeFrom = from
	m.themeErr = nil
	m.themeIndex = 0
	for i, name := range m.themeNames {
		if name == m.cfg.Theme {
			m.themeIndex = i
			break
		}
	}
	m.screen = screenThemePicker
	return m, nil
}

func (m *model) refreshSyncFor(name string) (tea.Model, tea.Cmd) {
	journalName, repoPath, err := m.cfg.journal(name)
	if err != nil {
		m.syncFor = name
		m.syncStatus = nil
		m.syncErr = err
		return m, nil
	}
	m.syncFor = journalName
	m.syncLoading = true
	m.syncStatus = nil
	m.syncErr = nil
	return m, tea.Batch(m.spinner.Tick, syncStatusCmd(journalName, repoPath))
}

func (m *model) switchActiveJournal(name string) error {
	if err := m.cfg.setActiveJournal(name); err != nil {
		return err
	}
	if err := writeConfig(m.cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	_, repoPath, err := m.cfg.activeJournal()
	if err != nil {
		return err
	}
	m.repoPath = repoPath
	for i, n := range m.journalNames {
		if n == name {
			m.journalIndex = i
			break
		}
	}
	return nil
}

func (m *model) setTheme(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("theme name cannot be empty")
	}
	if err := applyThemeByName(name); err != nil {
		return err
	}
	m.cfg.Theme = name
	if err := writeConfig(m.cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	m.spinner.Style = accentStyle
	m.list.spinner.Style = accentStyle
	return nil
}

func (m *model) selectedThemeName() string {
	if len(m.themeNames) == 0 {
		return ""
	}
	if m.themeIndex < 0 {
		m.themeIndex = 0
	}
	if m.themeIndex >= len(m.themeNames) {
		m.themeIndex = len(m.themeNames) - 1
	}
	return m.themeNames[m.themeIndex]
}

func (m model) syncSummary() string {
	label := "sync: "
	if m.syncFor != "" {
		label += m.syncFor + " · "
	}
	if m.syncLoading {
		return label + m.spinner.View() + " checking..."
	}
	if m.syncErr != nil {
		return label + "error: " + m.syncErr.Error()
	}
	if m.syncStatus == nil {
		return label + "unknown"
	}
	if !m.syncStatus.HasUpstream {
		if m.syncStatus.LocalOnly {
			return label + "local only"
		}
		return label + "no upstream configured"
	}
	switch {
	case m.syncStatus.Ahead == 0 && m.syncStatus.Behind == 0:
		return label + "up to date"
	case m.syncStatus.Ahead > 0 && m.syncStatus.Behind > 0:
		return fmt.Sprintf("%s%d to push, %d to pull", label, m.syncStatus.Ahead, m.syncStatus.Behind)
	case m.syncStatus.Ahead > 0:
		return fmt.Sprintf("%s%d to push", label, m.syncStatus.Ahead)
	default:
		return fmt.Sprintf("%s%d to pull", label, m.syncStatus.Behind)
	}
}

func formatDate(t time.Time) string {
	return t.Format("Mon, January 2 · 3:04 PM")
}

func formatComposeHeader(now time.Time, loc Location, locating bool, sp spinner.Model) string {
	header := formatDate(now)
	if location := loc.String(); location != "" {
		header += " · " + location
	} else if locating {
		header += " · " + sp.View() + " locating..."
	}
	return header
}

// ── List model ────────────────────────────────────────────────────────────────

type listState int

const (
	listLoading listState = iota
	listBrowsing
	listDetail
)

type entriesMsg struct {
	entries []Entry
	err     error
}

type listModel struct {
	state        listState
	spinner      spinner.Model
	entries      []Entry
	cursor       int
	top          int // first visible index in list
	viewport     viewport.Model
	windowHeight int
	windowWidth  int
	repoPath     string
	err          error
}

func newListModel(repoPath string) listModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accentStyle
	return listModel{
		state:    listLoading,
		spinner:  sp,
		repoPath: repoPath,
	}
}

func loadEntriesCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := loadEntries(repoPath)
		return entriesMsg{entries: entries, err: err}
	}
}

func (m listModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadEntriesCmd(m.repoPath))
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		if m.state == listDetail {
			m.viewport.Width = m.vpWidth()
			m.viewport.Height = m.vpHeight()
			m.viewport.SetContent(m.detailContent())
		}
		return m, nil

	case entriesMsg:
		m.err = msg.err
		m.entries = msg.entries
		m.state = listBrowsing
		return m, nil

	case spinner.TickMsg:
		if m.state == listLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.state == listDetail {
				m.state = listBrowsing
				return m, nil
			}
			return m, tea.Quit
		}

		if m.state == listBrowsing {
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.top = clampTop(m.cursor, m.top, m.visibleCount())
				}
			case "down", "j":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.top = clampTop(m.cursor, m.top, m.visibleCount())
				}
			case "enter", " ":
				if len(m.entries) > 0 {
					m.state = listDetail
					m.viewport = viewport.New(m.vpWidth(), m.vpHeight())
					m.viewport.SetContent(m.detailContent())
				}
			}
			return m, nil
		}

		if m.state == listDetail {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	if m.state == listDetail {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(renderHead("journal", "entries") + "\n")

	switch m.state {
	case listLoading:
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, m.spinner.View()+" "+subtitleStyle.Render("loading entries...")), "  ") + "\n")

	case listBrowsing:
		var body strings.Builder
		if m.err != nil {
			body.WriteString(errorStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n")
		} else if len(m.entries) == 0 {
			body.WriteString(subtitleStyle.Render("no entries found for this journal") + "\n")
		} else {
			end := m.top + m.visibleCount()
			if end > len(m.entries) {
				end = len(m.entries)
			}
			for i := m.top; i < end; i++ {
				e := m.entries[i]
				line := formatDate(e.Timestamp)
				if e.Location != "" {
					line += " · " + e.Location
				}
				if i == m.cursor {
					body.WriteString(accentStyle.Render("▶") + " " + selectedStyle.Render(line) + "\n")
				} else {
					body.WriteString("  " + hintStyle.Render(line) + "\n")
				}
			}
		}
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, strings.TrimRight(body.String(), "\n")), "  ") + "\n")
		b.WriteString("  " + hintStyle.Render("↑↓ navigate  ·  enter view  ·  q quit") + "\n")

	case listDetail:
		var body strings.Builder
		if m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			body.WriteString(sectionLabelStyle.Render(formatDate(e.Timestamp)) + "\n")
			if e.Location != "" {
				body.WriteString(subtitleStyle.Render(e.Location) + "\n")
			}
			sep := strings.Repeat("─", m.vpWidth())
			body.WriteString(hintStyle.Render(sep) + "\n\n")
		}
		for _, line := range strings.Split(m.viewport.View(), "\n") {
			body.WriteString(line + "\n")
		}
		b.WriteString(indentBlock(renderPanelWithWidth(m.windowWidth, strings.TrimRight(body.String(), "\n")), "  ") + "\n")
		scrollPct := ""
		if m.viewport.TotalLineCount() > m.viewport.VisibleLineCount() {
			scrollPct = fmt.Sprintf(" (%d%%)", int(m.viewport.ScrollPercent()*100))
		}
		b.WriteString("  " + hintStyle.Render("↑↓ scroll"+scrollPct+"  ·  esc back  ·  q quit") + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m listModel) visibleCount() int {
	// title(1) + blank(1) + footer blank(1) + footer hint(1) = 4
	v := m.windowHeight - 4
	if v < 3 {
		v = 3
	}
	return v
}

func (m listModel) vpWidth() int {
	w := m.windowWidth - 6 // account for panel border/padding + left indent
	if w < 1 {
		w = 1
	}
	return w
}

func (m listModel) vpHeight() int {
	// title(1) + blank(1) + date(1) + location(1) + sep(1) + blank(1) + footer blank(1) + footer hint(1) = 8
	h := m.windowHeight - 8
	if h < 1 {
		h = 1
	}
	return h
}

func (m listModel) detailContent() string {
	if m.cursor >= len(m.entries) {
		return ""
	}
	return wordwrap.String(m.entries[m.cursor].Body, m.vpWidth())
}

func clampTop(cursor, top, visible int) int {
	if cursor < top {
		return cursor
	}
	if cursor >= top+visible {
		return cursor - visible + 1
	}
	return top
}
