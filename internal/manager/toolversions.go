package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// ToolVersions reads asdf/mise `.tool-versions` files.
type ToolVersions struct{}

func (ToolVersions) Name() string { return "tool-versions" }

func (ToolVersions) Match(path string) bool { return filepath.Base(path) == ".tool-versions" }

// Extract parses .tool-versions per asdf/mise format. Each non-comment line is
//
//	<tool> <version> [<version>...]
//
// We emit one Entry per (tool, version). The Name column carries the tool
// identifier so the file column alone doesn't collapse all tools into
// "asdf/mise".
func (ToolVersions) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}

	var entries []entry.Entry
	dir := filepath.Dir(file.Path)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		raw := scanner.Text()
		if i := strings.Index(raw, "#"); i >= 0 {
			raw = raw[:i]
		}
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tool := fields[0]
		for _, ver := range fields[1:] {
			entries = append(entries, entry.Entry{
				Type:    typeFromTool(tool),
				Path:    dir,
				Name:    ".tool-versions",
				Version: ver,
			})
		}
	}
	return entries, scanner.Err()
}
