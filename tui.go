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

// Messages
type geoMsg struct {
	loc Location
	err error
}

type commitDoneMsg struct {
	err error
}

// Styles
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
)

type model struct {
	state    state
	spinner  spinner.Model
	textarea textarea.Model
	locating bool
	location Location
	now      time.Time
	repoPath string
	err      error
}

func newModel(repoPath string) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

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
		state:    stateComposing,
		spinner:  sp,
		textarea: ta,
		locating: true,
		now:      time.Now(),
		repoPath: repoPath,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchGeoCmd(),
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
		err := commitEntry(repoPath, entry, loc)
		return commitDoneMsg{err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
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

	case geoMsg:
		m.locating = false
		if msg.err == nil {
			m.location = msg.loc
		}
		return m, textarea.Blink

	case commitDoneMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
		} else {
			m.state = stateDone
		}
		return m, tea.Quit

	case spinner.TickMsg:
		if m.locating || m.state == stateCommitting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if m.state == stateComposing {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("journal") + "\n")

	switch m.state {
	case stateComposing:
		b.WriteString("  " + subtitleStyle.Render(formatComposeHeader(m.now, m.location, m.locating, m.spinner)) + "\n")
		b.WriteString("\n")
		// Indent textarea
		lines := strings.Split(m.textarea.View(), "\n")
		for _, line := range lines {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("\n")
		b.WriteString("  " + hintStyle.Render("ctrl+s commit  ·  esc quit") + "\n")

	case stateCommitting:
		header := formatDate(m.now)
		if location := m.location.String(); location != "" {
			header += " · " + location
		}
		b.WriteString("  " + subtitleStyle.Render(header) + "\n")
		b.WriteString("\n")
		b.WriteString("  " + m.spinner.View() + " committing…\n")

	case stateDone:
		b.WriteString("\n")
		b.WriteString("  " + successStyle.Render("entry saved.") + "\n")

	case stateError:
		b.WriteString("\n")
		b.WriteString("  " + errorStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n")
		b.WriteString("\n")
		b.WriteString("  " + hintStyle.Render("press any key to exit") + "\n")
	}

	b.WriteString("\n")
	return b.String()
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
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
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
	b.WriteString("  " + titleStyle.Render("journal") + " " + hintStyle.Render("— entries") + "\n")
	b.WriteString("\n")

	switch m.state {
	case listLoading:
		b.WriteString("  " + m.spinner.View() + " loading entries…\n")

	case listBrowsing:
		if m.err != nil {
			b.WriteString("  " + errorStyle.Render(fmt.Sprintf("error: %v", m.err)) + "\n")
		} else if len(m.entries) == 0 {
			b.WriteString("  " + subtitleStyle.Render("no entries yet") + "\n")
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
					b.WriteString("  " + accentStyle.Render("▶") + " " + selectedStyle.Render(line) + "\n")
				} else {
					b.WriteString("    " + hintStyle.Render(line) + "\n")
				}
			}
		}
		b.WriteString("\n")
		b.WriteString("  " + hintStyle.Render("↑↓ navigate  ·  enter view  ·  q quit") + "\n")

	case listDetail:
		if m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			b.WriteString("  " + subtitleStyle.Render(formatDate(e.Timestamp)) + "\n")
			if e.Location != "" {
				b.WriteString("  " + hintStyle.Render(e.Location) + "\n")
			}
			sep := strings.Repeat("─", m.vpWidth())
			b.WriteString("  " + hintStyle.Render(sep) + "\n")
			b.WriteString("\n")
		}
		for _, line := range strings.Split(m.viewport.View(), "\n") {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("\n")
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
	w := m.windowWidth - 4
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
