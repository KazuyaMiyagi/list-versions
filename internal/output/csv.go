package output

import (
	"encoding/csv"
	"io"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

func EntriesCSV(w io.Writer, es []entry.Entry, keys []string) error {
	cw := csv.NewWriter(w)
	headers := make([]string, len(keys))
	for i, k := range keys {
		headers[i] = Header(k)
	}
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, e := range es {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = fieldByKey(e, k)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
