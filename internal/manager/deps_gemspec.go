package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// The name must be a literal right after the method name. Taking the first
// literal anywhere on the line would, for a constant name, shift a constraint
// into the name slot.
var gemspecDepRe = regexp.MustCompile(`^\s*\w+\.add_(?:runtime_|development_)?dependency[\s(]*((?:"|'|%q<).*)$`)

// GemspecDeps surfaces the add_dependency declarations of a *.gemspec file.
// Like GemfileDeps, the VERSION is the declared constraint, not a resolved
// version.
type GemspecDeps struct{}

func (GemspecDeps) Name() string { return "gemspec-deps" }

func (GemspecDeps) Match(path string) bool { return filepath.Ext(path) == ".gemspec" }

func (GemspecDeps) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		m := gemspecDepRe.FindStringSubmatch(scanner.Text())
		if m == nil || strings.Contains(m[1], "#{") {
			continue
		}
		lits := rubyStringLiterals(m[1])
		if len(lits) == 0 {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "gem:" + lits[0],
			Path:    dir,
			Name:    filepath.Base(file.Path),
			Version: strings.Join(lits[1:], ", "),
		})
	}
	return entries, scanner.Err()
}
