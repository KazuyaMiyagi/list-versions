package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i >= len(widths) {
				break // ignore cells with no corresponding header
			}
			if n := utf8.RuneCountInString(c); n > widths[i] {
				widths[i] = n
			}
		}
	}

	border := func(l, m, r string) string {
		var b strings.Builder
		b.WriteString(l)
		for i, wd := range widths {
			b.WriteString(strings.Repeat("─", wd+2))
			if i < len(widths)-1 {
				b.WriteString(m)
			}
		}
		b.WriteString(r)
		return b.String()
	}

	writeRow := func(cells []string) {
		var b strings.Builder
		b.WriteString("│")
		for i, c := range cells {
			if i >= len(widths) {
				break
			}
			pad := widths[i] - utf8.RuneCountInString(c)
			b.WriteString(" ")
			b.WriteString(c)
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(" │")
		}
		fmt.Fprintln(w, b.String())
	}

	fmt.Fprintln(w, border("┌", "┬", "┐"))
	writeRow(headers)
	fmt.Fprintln(w, border("├", "┼", "┤"))
	for _, r := range rows {
		writeRow(r)
	}
	fmt.Fprintln(w, border("└", "┴", "┘"))
	return nil
}

func EntriesTable(w io.Writer, es []entry.Entry, keys []string) error {
	headers := make([]string, len(keys))
	for i, k := range keys {
		headers[i] = Header(k)
	}
	rows := make([][]string, 0, len(es))
	for _, e := range es {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = fieldByKey(e, k)
		}
		rows = append(rows, row)
	}
	return WriteTable(w, headers, rows)
}
