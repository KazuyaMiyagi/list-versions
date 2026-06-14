package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// gemDirectiveRe matches `gem "name", "constraint" [, ...]` lines. The
// constraint is optional; when absent we still record the dependency with an
// empty VERSION so it shows up in the inventory.
var gemDirectiveRe = regexp.MustCompile(`^\s*gem\s+["']([^"']+)["'](?:\s*,\s*["']([^"']+)["'])?`)

// GemfileDeps surfaces every `gem "x", "..."` line as an entry. The output
// reflects what the developer wrote in Gemfile, which is a constraint rather
// than the resolved version. Pair it with GemfileLockDeps to get the resolved
// versions side-by-side.
type GemfileDeps struct{}

func (GemfileDeps) Name() string { return "gemfile-deps" }

func (GemfileDeps) Match(path string) bool { return filepath.Base(path) == "Gemfile" }

func (GemfileDeps) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		m := gemDirectiveRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "gem:" + m[1],
			Path:    dir,
			Name:    "Gemfile",
			Version: m[2],
		})
	}
	return entries, scanner.Err()
}

// GemfileLockDeps parses Gemfile.lock and emits one entry per gem with the
// resolved version. The lockfile lists transitive gems too; we keep all of
// them because they reflect what `bundle install` actually pinned.
type GemfileLockDeps struct{}

func (GemfileLockDeps) Name() string { return "gemfile-lock-deps" }

func (GemfileLockDeps) Match(path string) bool { return filepath.Base(path) == "Gemfile.lock" }

// gemSpecRe matches `    gem-name (1.2.3)` indented under `specs:` in
// Gemfile.lock. Anything else (sub-dependencies, RUBY VERSION, BUNDLED WITH)
// uses a different indent or no parentheses, so it's filtered out cleanly.
var gemSpecRe = regexp.MustCompile(`^    ([a-zA-Z0-9._-]+)\s+\(([^)]+)\)\s*$`)

func (GemfileLockDeps) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	inSpecs := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "specs:" {
			inSpecs = true
			continue
		}
		// Section header (e.g. PLATFORMS, DEPENDENCIES, RUBY VERSION,
		// BUNDLED WITH) starts at column 0 with no leading space.
		if line != "" && !strings.HasPrefix(line, " ") {
			inSpecs = false
			continue
		}
		if !inSpecs {
			continue
		}
		m := gemSpecRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "gem:" + m[1],
			Path:    dir,
			Name:    "Gemfile.lock",
			Version: m[2],
		})
	}
	return entries, scanner.Err()
}
