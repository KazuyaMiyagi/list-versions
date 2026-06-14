package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// GoMod reads go.mod and emits one entry per direct `require` line. Lines
// marked `// indirect` (transitive deps the go tool tracks for build
// reproducibility) are skipped because they're not part of the developer's
// stated dependency set.
type GoMod struct{}

func (GoMod) Name() string { return "go-mod" }

func (GoMod) Match(path string) bool { return filepath.Base(path) == "go.mod" }

func (GoMod) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))

	inRequireBlock := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			inRequireBlock = true
			continue
		case inRequireBlock && trimmed == ")":
			inRequireBlock = false
			continue
		}

		var body string
		if inRequireBlock {
			body = trimmed
		} else if strings.HasPrefix(trimmed, "require ") {
			body = strings.TrimPrefix(trimmed, "require ")
		} else {
			continue
		}

		// Drop comments (and skip `// indirect` lines).
		if strings.Contains(body, "// indirect") {
			continue
		}
		if i := strings.Index(body, "//"); i >= 0 {
			body = strings.TrimSpace(body[:i])
		}
		fields := strings.Fields(body)
		if len(fields) < 2 {
			continue
		}
		entries = append(entries, entry.Entry{
			Type: "go:" + fields[0], Path: dir, Name: "go.mod", Version: fields[1],
		})
	}
	return entries, scanner.Err()
}
