package ui

import (
	"bytes"
	"fmt"
	"os"

	fp "github.com/charmbracelet/bubbles/filepicker"
	ti "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"testme/parser"
	tpl "testme/template"
)

/* ============== DATA & STATE ============== */

type editMode int

var censusYears = []string{"1841", "1851", "1861", "1871", "1881", "1891", "1901", "1911", "1921"}

const (
	modeYearSelect editMode = iota
	modeHeader
	modeBody
	modeFooter
	modePickFile
)

var modeNames = []string{"YEAR", "HEADER", "BODY", "FOOTER"}

type Row = parser.Row

type model struct {
	// persistent data
	header [parser.HeadCount]string
	rows   [parser.RowCount]Row
	footer [parser.FootCount]string

	// year selection
	year    string
	yearIdx int

	// editing state
	mode      editMode
	currRow   int // only for body
	currCol   int
	justWrote bool
	justRead  bool

	// widgets
	headIn [parser.HeadCount]ti.Model
	bodyIn [parser.FieldCount]ti.Model
	footIn [parser.FootCount]ti.Model
	picker fp.Model
}

func newInput(ph string) ti.Model {
	in := ti.New()
	in.Placeholder = ph
	in.CharLimit = 64
	return in
}

func NewModel() model {
	m := model{}

	headLbl := []string{"Parish", "City", "Ward", "Parl Borough", "Town", "Hamlet", "Ecc District"}
	bodyLbl := []string{
		"Sched#", "Road / House", "Inhab.", "Uninh.",
		"Name & Surname", "Relation", "Condition",
		"Age♂", "Age♀", "Occupation", "Where born", "Blind/Deaf",
	}
	footLbl := []string{"Houses Inhab", "Houses Uninh", "Total Males", "Total Females"}

	for i := range m.headIn {
		m.headIn[i] = newInput(headLbl[i])
	}
	for i := range m.bodyIn {
		m.bodyIn[i] = newInput(bodyLbl[i])
	}
	for i := range m.footIn {
		m.footIn[i] = newInput(footLbl[i])
	}
	m.headIn[0].Focus()

	// file-picker
	p := fp.New()
	p.AllowedTypes = []string{".html", ".htm"}
	m.picker = p

	m.mode = modeYearSelect
	m.yearIdx = 2 // default to 1861

	return m
}

/* ============== TEA ============== */

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.justWrote, m.justRead = false, false

	/* ---------- YEAR SELECT MODE ----------- */
	if m.mode == modeYearSelect {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyUp:
				m.yearIdx = (m.yearIdx - 1 + len(censusYears)) % len(censusYears)
			case tea.KeyDown:
				m.yearIdx = (m.yearIdx + 1) % len(censusYears)
			case tea.KeyEnter:
				m.year = censusYears[m.yearIdx]
				m.mode = modeHeader
				m.loadCurrent()
			}
		}
		return m, nil
	}

	/* ---------- FILE‑PICKER MODE ---------- */
	if m.mode == modePickFile {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)

		if didSel, path := m.picker.DidSelectFile(msg); didSel {
			if err := m.loadFromHTML(path); err == nil {
				m.justRead = true
			} else {
				fmt.Fprintf(os.Stderr, "load error: %v\n", err)
			}
			m.mode = modeHeader
			return m, nil
		}

		if km, ok := msg.(tea.KeyMsg); ok && (km.Type == tea.KeyEsc || km.Type == tea.KeyCtrlC) {
			m.mode = modeHeader
			return m, nil
		}
		return m, cmd
	}

	/* ---------- EDITING MODES ------------- */
	switch k := msg.(type) {
	case tea.KeyMsg:
		switch k.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlH:
			m.switchMode(modeHeader)
		case tea.KeyCtrlB:
			m.switchMode(modeBody)
		case tea.KeyCtrlF:
			m.switchMode(modeFooter)
		case tea.KeyCtrlO:
			m.commitCurrent()
			m.mode = modePickFile
			return m, m.picker.Init()
		case tea.KeyTab:
			m.currCol++
			m.wrapCol()
			m.setFocus()
		case tea.KeyShiftTab:
			m.currCol--
			m.wrapCol()
			m.setFocus()
		case tea.KeyUp:
			if m.mode == modeBody && m.currRow > 0 {
				m.commitCurrent()
				m.currRow--
				m.loadCurrent()
			}
		case tea.KeyDown:
			if m.mode == modeBody && m.currRow < parser.RowCount-1 {
				m.commitCurrent()
				m.currRow++
				m.loadCurrent()
			}
		case tea.KeyCtrlN:
			if m.mode == modeBody {
				for i := range m.bodyIn {
					m.bodyIn[i].SetValue("")
				}
			}
		case tea.KeyCtrlW:
			m.commitCurrent()
			if err := tpl.WriteHTML(m.header, m.rows[:], m.footer, "census.html"); err == nil {
				m.justWrote = true
			} else {
				fmt.Fprintf(os.Stderr, "save error: %v\n", err)
			}
		}

		// pass key to focused input
		switch m.mode {
		case modeHeader:
			m.headIn[m.currCol], _ = m.headIn[m.currCol].Update(k)
		case modeBody:
			m.bodyIn[m.currCol], _ = m.bodyIn[m.currCol].Update(k)
		case modeFooter:
			m.footIn[m.currCol], _ = m.footIn[m.currCol].Update(k)
		}
	}
	return m, nil
}

/* ---------- helpers ---------- */

func (m *model) switchMode(next editMode) {
	m.commitCurrent()
	m.mode, m.currCol = next, 0
	m.loadCurrent()
}

func (m *model) wrapCol() {
	switch m.mode {
	case modeHeader:
		m.currCol = (m.currCol + parser.HeadCount) % parser.HeadCount
	case modeBody:
		m.currCol = (m.currCol + parser.FieldCount) % parser.FieldCount
	case modeFooter:
		m.currCol = (m.currCol + parser.FootCount) % parser.FootCount
	}
}

/* ============== VIEW ============== */

func (m model) View() string {
	if m.mode == modeYearSelect {
		var b bytes.Buffer
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Select census year:\n\n"))
		for i, y := range censusYears {
			cursor := " "
			if i == m.yearIdx {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, y))
		}
		b.WriteString("\n(↑/↓ to choose, Enter to select, Esc to quit)")
		return b.String()
	}

	if m.mode == modePickFile {
		return lipgloss.NewStyle().Bold(true).Render("Pick a census HTML file (Esc to cancel):\n\n") + m.picker.View()
	}

	var b bytes.Buffer
	year := m.year
	if year == "" {
		year = "1861"
	}
	title := fmt.Sprintf(
		"%s Census TUI — %-6s  (Ctrl‑H/B/F • ↑↓ • Tab/Shift‑Tab • Ctrl‑N clear row • Ctrl‑O open • Ctrl‑W write • Esc)",
		year, modeNames[m.mode],
	)
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(title) + "\n\n")

	lbl := lipgloss.NewStyle().Padding(0, 1)
	printInputs := func(list []ti.Model) {
		for _, in := range list {
			b.WriteString(lbl.Render(in.Placeholder) + in.View() + "\n")
		}
	}

	switch m.mode {
	case modeHeader:
		printInputs(m.headIn[:])
	case modeBody:
		b.WriteString(lipgloss.NewStyle().Italic(true).Render(fmt.Sprintf("(Row %d of 25)\n\n", m.currRow+1)))
		printInputs(m.bodyIn[:])
	case modeFooter:
		printInputs(m.footIn[:])
	}

	if m.justWrote {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ census.html written"))
	}
	if m.justRead {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ HTML loaded"))
	}
	return b.String()
}

/* ============== PERSISTENCE ============== */

func (m *model) commitCurrent() {
	switch m.mode {
	case modeHeader:
		for i := range m.headIn {
			m.header[i] = m.headIn[i].Value()
		}
	case modeBody:
		for i := range m.bodyIn {
			m.rows[m.currRow].Col[i] = m.bodyIn[i].Value()
		}
	case modeFooter:
		for i := range m.footIn {
			m.footer[i] = m.footIn[i].Value()
		}
	}
}

func (m *model) loadCurrent() {
	switch m.mode {
	case modeHeader:
		for i := range m.headIn {
			m.headIn[i].SetValue(m.header[i])
		}
	case modeBody:
		for i := range m.bodyIn {
			m.bodyIn[i].SetValue(m.rows[m.currRow].Col[i])
		}
	case modeFooter:
		for i := range m.footIn {
			m.footIn[i].SetValue(m.footer[i])
		}
	}
	m.setFocus()
}

func (m *model) setFocus() {
	for i := range m.headIn {
		if m.mode == modeHeader && i == m.currCol {
			m.headIn[i].Focus()
		} else {
			m.headIn[i].Blur()
		}
	}
	for i := range m.bodyIn {
		if m.mode == modeBody && i == m.currCol {
			m.bodyIn[i].Focus()
		} else {
			m.bodyIn[i].Blur()
		}
	}
	for i := range m.footIn {
		if m.mode == modeFooter && i == m.currCol {
			m.footIn[i].Focus()
		} else {
			m.footIn[i].Blur()
		}
	}
}

/* ============== HTML IO ============== */

func (m *model) loadFromHTML(path string) error {
	h, r, f, err := parser.ParseHTML(path)
	if err != nil {
		return err
	}
	m.header, m.rows, m.footer = h, r, f
	m.currRow, m.currCol = 0, 0
	m.loadCurrent()
	return nil
}

/* ============== PROGRAM ============== */

// Start launches the Bubble Tea program using this model.
func Start() error {
	return tea.NewProgram(NewModel()).Start()
}
