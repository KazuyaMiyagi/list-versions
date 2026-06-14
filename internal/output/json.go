package output

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// EntriesJSON emits one object per entry. The keys in each object appear in
// the same order as `keys`, mirroring the table/csv column order so the JSON
// output stays predictable for downstream tooling.
func EntriesJSON(w io.Writer, es []entry.Entry, keys []string) error {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, e := range es {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n  {")
		for j, k := range keys {
			if j > 0 {
				buf.WriteString(", ")
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			vb, err := json.Marshal(fieldByKey(e, k))
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteString(": ")
			buf.Write(vb)
		}
		buf.WriteString("}")
	}
	if len(es) > 0 {
		buf.WriteString("\n")
	}
	buf.WriteString("]\n")
	_, err := io.Copy(w, &buf)
	return err
}
