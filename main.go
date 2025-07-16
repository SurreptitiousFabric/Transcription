// main.go — 1861 Census TUI
//
// Key bindings
//   Ctrl‑H / Ctrl‑B / Ctrl‑F   – switch between header, body and footer areas
//   ↑ / ↓  and Tab / Shift‑Tab – move between inputs
//   Ctrl‑N                     – clear current body row
//   Ctrl‑W                     – write census.html
//   Ctrl‑O                     – open an existing census.html (file‑picker)
//   Esc / Ctrl‑C               – quit
//
// Setup
//   go mod init census
//   go get github.com/charmbracelet/bubbletea@v0.25.0
//   go get github.com/charmbracelet/bubbles/filepicker@v0.21.0
//   go get github.com/charmbracelet/bubbles/textinput@v0.15.0
//   go get github.com/charmbracelet/lipgloss@v0.9.1
//   go get golang.org/x/net/html
//   go run main.go

package main

import (
	"bytes"
	"fmt"
	htmlstd "html" // stdlib html (EscapeString)
	"html/template"
	"os"
	"path/filepath"
	"strings"

	fp "github.com/charmbracelet/bubbles/filepicker"
	ti "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/html" // DOM parser
)

/* ============== CONFIG ============== */

const (
	rowCount   = 25
	fieldCount = 12
	headCount  = 7
	footCount  = 4
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

type Row struct{ Col [fieldCount]string }

type model struct {
	// persistent data
	header [headCount]string
	rows   [rowCount]Row
	footer [footCount]string

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
	headIn [headCount]ti.Model
	bodyIn [fieldCount]ti.Model
	footIn [footCount]ti.Model
	picker fp.Model
}

func newInput(ph string) ti.Model {
	in := ti.New()
	in.Placeholder = ph
	in.CharLimit = 64
	return in
}

func newModel() model {
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
			if m.mode == modeBody && m.currRow < rowCount-1 {
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
			if err := writeHTML(m.header, m.rows[:], m.footer, "census.html"); err == nil {
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
		m.currCol = (m.currCol + headCount) % headCount
	case modeBody:
		m.currCol = (m.currCol + fieldCount) % fieldCount
	case modeFooter:
		m.currCol = (m.currCol + footCount) % footCount
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

/* ============== HTML OUTPUT ============== */

func wrapCell(v string, row, col int) template.HTML {
	if v == "" {
		return ""
	}
	esc := htmlstd.EscapeString(v)
	r, c := row+1, col+1
	switch col {
	case 4:
		return template.HTML(fmt.Sprintf(`<PersonRef detlnk="dpR%dC%d">%s</PersonRef>`, r, c, esc))
	case 1, 10:
		return template.HTML(fmt.Sprintf(`<PlaceRef detlnk="dwR%dC%d">%s</PlaceRef>`, r, c, esc))
	default:
		return template.HTML(fmt.Sprintf(`<Mark ref="R%dC%d">%s</Mark>`, r, c, esc))
	}
}

func headerVal(v string) template.HTML {
	if v == "" {
		return ""
	}
	return template.HTML("<br>" + htmlstd.EscapeString(v))
}

type pageData struct {
	Header [headCount]string
	Rows   []Row
	Footer [footCount]string
}

const pageTmpl = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>1861 Census</title>
<style>
  .smaller-header { font-size: 8px; }
  .small-header   { font-size: 10px; }
  table, th, td   { border: 1px solid black; border-collapse: collapse; }
  th, td          { padding: 4px; font-size: 12px; }
  thead th        { background-color: #f0f0f0; }
</style>
</head>
<body>
<!-- HEADER -->
<table border="1" cellspacing="0" cellpadding="0">
  <colgroup><col style="width:8.33%" span="7"></colgroup>
  <thead>
    <tr><th colspan="7" align="center">
      The undermentioned Houses are situate within the Boundaries of the
    </th></tr>
    <tr>
      <th style="line-height:5em; padding-bottom:2em;">Parish [or Township] of{{headerVal (index .Header 0)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">City or Municipal Borough of{{headerVal (index .Header 1)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Municipal Ward of{{headerVal (index .Header 2)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Parliamentary Borough of{{headerVal (index .Header 3)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Town of{{headerVal (index .Header 4)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Hamlet or Tything, &amp;c., of{{headerVal (index .Header 5)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Ecclesiastical District of{{headerVal (index .Header 6)}}</th>
    </tr>
  </thead>
</table>

<!-- BODY -->
<table>
  <colgroup>
    <col style="width:1%"><col style="width:8%"><col style="width:.5%"><col style="width:.5%">
    <col style="width:15%"><col style="width:8%"><col style="width:2%">
    <col style="width:1.5%"><col style="width:1.5%"><col style="width:20%">
    <col style="width:15%"><col style="width:4%">
  </colgroup>
  <thead>
    <tr>
      <th class="smaller-header" rowspan="2">No.<br>Schedule</th>
      <th class="small-header"   rowspan="2">Road, Street, &amp;c;<br>&amp; No. or Name of House</th>
      <th class="smaller-header" colspan="2">HOUSES</th>
      <th class="small-header"   rowspan="2">Name and Surname of each Person</th>
      <th rowspan="2">Relation</th><th rowspan="2">Condition</th>
      <th colspan="2">Age of</th>
      <th rowspan="2">Rank, Profession, or Occupation</th>
      <th rowspan="2">Where Born</th>
      <th class="smaller-header" rowspan="2">Blind or Deaf-Dumb</th>
    </tr>
    <tr>
      <th class="smaller-header">In-habited</th><th class="smaller-header">Un-inhabited or Bldg.</th>
      <th class="smaller-header">Males</th><th class="smaller-header">Females</th>
    </tr>
  </thead>
  <tbody>
    {{range $ri, $row := .Rows}}
    <tr>{{range $ci, $val := $row.Col}}<td>{{wrapCell $val $ri $ci}}</td>{{end}}</tr>
    {{end}}
  </tbody>
  <!-- FOOTER -->
  <tfoot>
    <tr>
      <td colspan="2" align="right">Total of Houses...</td>
      <td>{{index .Footer 0}}</td>
      <td>{{index .Footer 1}}</td>
      <td colspan="3" align="right">Total of Males and Females...</td>
      <td>{{index .Footer 2}}</td>
      <td>{{index .Footer 3}}</td>
      <td colspan="2"></td><td></td>
    </tr>
  </tfoot>
</table>
</body>
</html>`

func writeHTML(header [headCount]string, rows []Row, footer [footCount]string, filename string) error {
	data := pageData{Header: header, Rows: rows, Footer: footer}
	t := template.Must(template.New("page").Funcs(template.FuncMap{
		"wrapCell":  wrapCell,
		"headerVal": headerVal,
	}).Parse(pageTmpl))

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0o644)
}

/* ============== HTML INPUT ============== */

func (m *model) loadFromHTML(path string) error {
	h, r, f, err := parseHTML(path)
	if err != nil {
		return err
	}
	m.header, m.rows, m.footer = h, r, f
	m.currRow, m.currCol = 0, 0
	m.loadCurrent()
	return nil
}

func parseHTML(path string) ([headCount]string, [rowCount]Row, [footCount]string, error) {
	var head [headCount]string
	var rows [rowCount]Row
	var foot [footCount]string

	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return head, rows, foot, err
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return head, rows, foot, err
	}

	// helper functions
	text := func(n *html.Node) string {
		if n == nil {
			return ""
		}
		var sb strings.Builder
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.TextNode {
				sb.WriteString(n.Data)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(n)
		return strings.TrimSpace(sb.String())
	}
	is := func(n *html.Node, tag string) bool { return n != nil && n.Type == html.ElementNode && n.Data == tag }

	/* ---- header ---- */
	var ths []*html.Node
	var collectTh func(*html.Node)
	collectTh = func(n *html.Node) {
		if is(n, "th") {
			ths = append(ths, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectTh(c)
		}
	}
	collectTh(doc)

	idx := 0
	for _, th := range ths {
		if idx >= headCount {
			break
		}
		for c := th.FirstChild; c != nil; c = c.NextSibling {
			if is(c, "br") {
				head[idx] = strings.TrimSpace(text(c.NextSibling))
				idx++
				break
			}
		}
	}

	/* ---- body rows ---- */
	var trs []*html.Node
	var collectTr func(*html.Node)
	collectTr = func(n *html.Node) {
		if is(n, "tr") && ancestorTag(n, "tbody") {
			trs = append(trs, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectTr(c)
		}
	}
	collectTr(doc)

	for ri := 0; ri < len(trs) && ri < rowCount; ri++ {
		td := trs[ri].FirstChild
		ci := 0
		for td != nil && ci < fieldCount {
			if is(td, "td") {
				rows[ri].Col[ci] = text(td)
				ci++
			}
			td = td.NextSibling
		}
	}

	/* ---- footer ---- */
	var footvals []string
	var walkFooter func(*html.Node)
	walkFooter = func(n *html.Node) {
		if is(n, "td") && ancestorTag(n, "tfoot") {
			for _, a := range n.Attr {
				if a.Key == "colspan" {
					return
				}
			}
			footvals = append(footvals, text(n))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkFooter(c)
		}
	}
	walkFooter(doc)
	for i := 0; i < len(footvals) && i < footCount; i++ {
		foot[i] = footvals[i]
	}

	return head, rows, foot, nil
}

func ancestorTag(n *html.Node, tag string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == tag {
			return true
		}
	}
	return false
}

/* ============== MAIN ============== */

func main() {
	if err := tea.NewProgram(newModel()).Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
