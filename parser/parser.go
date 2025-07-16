package parser

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

const (
	RowCount   = 25
	FieldCount = 12
	HeadCount  = 7
	FootCount  = 4
)

type Row struct{ Col [FieldCount]string }

// ParseHTML reads the census HTML at path and returns header, body rows and footer values.
func ParseHTML(path string) ([HeadCount]string, [RowCount]Row, [FootCount]string, error) {
	var head [HeadCount]string
	var rows [RowCount]Row
	var foot [FootCount]string

	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return head, rows, foot, err
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return head, rows, foot, err
	}

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

	// header
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
		if idx >= HeadCount {
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

	// body
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

	for ri := 0; ri < len(trs) && ri < RowCount; ri++ {
		td := trs[ri].FirstChild
		ci := 0
		for td != nil && ci < FieldCount {
			if is(td, "td") {
				rows[ri].Col[ci] = text(td)
				ci++
			}
			td = td.NextSibling
		}
	}

	// footer
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
	for i := 0; i < len(footvals) && i < FootCount; i++ {
		foot[i] = footvals[i]
	}

	return head, rows, foot, nil
}

// ancestorTag reports whether n has an ancestor element with the given tag name.
func ancestorTag(n *html.Node, tag string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == tag {
			return true
		}
	}
	return false
}
