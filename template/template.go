package template

import (
	"bytes"
	"fmt"
	htmlstd "html"
	"html/template"
	"os"

	"testme/parser"
)

// wrapCell formats a body cell with HTML markup when saving.
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
	Header [parser.HeadCount]string
	Rows   []parser.Row
	Footer [parser.FootCount]string
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
      <th style="line-height:5em; padding-bottom:2em;">Village or Hamlet of{{headerVal (index .Header 5)}}</th>
      <th style="line-height:5em; padding-bottom:2em;">Ecclesiastical District of{{headerVal (index .Header 6)}}</th>
    </tr>
    <tr>
      <th class="small-header">Sched. No.</th>
      <th class="small-header">Road, Street, &amp; No. or Name of House</th>
      <th class="small-header">Houses</th>
      <th class="smaller-header">Name &amp; Surname of each Person</th>
      <th class="smaller-header">Relation to Head of Family</th>
      <th class="smaller-header">Condition</th>
      <th class="smaller-header">Age of</th>
      <th class="small-header">Age of</th>
      <th class="small-header">Rank, Profession, or Occupation</th>
      <th class="small-header">Where Born</th>
      <th class="small-header">Whether Blind or Deaf-and-Dumb</th>
      <th class="small-header"></th>
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

// WriteHTML renders the census data to an HTML file.
func WriteHTML(header [parser.HeadCount]string, rows []parser.Row, footer [parser.FootCount]string, filename string) error {
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
