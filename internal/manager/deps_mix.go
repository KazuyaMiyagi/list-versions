package manager

import (
	"path/filepath"
	"regexp"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// MixExs reads Elixir's mix.exs. We don't parse the full Elixir syntax, just
// scan for `{:name, "constraint"}` tuples — the canonical shape for entries
// inside `defp deps do [...]`.
type MixExs struct{}

func (MixExs) Name() string { return "mix-exs" }

func (MixExs) Match(path string) bool { return filepath.Base(path) == "mix.exs" }

// mixDepRe matches `{:name, "constraint"}` with optional whitespace.
var mixDepRe = regexp.MustCompile(`\{\s*:([a-z_][a-zA-Z0-9_]*)\s*,\s*"([^"]+)"`)

func (MixExs) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for _, m := range mixDepRe.FindAllStringSubmatch(string(data), -1) {
		name, ver := m[1], m[2]
		// Skip values that look like file/git/path options ({:dep, "github: ..."})
		// by requiring the second arg to look like a SemVer-ish version
		// constraint (begins with a digit, operator, or 'v').
		if !looksLikeVersion(ver) {
			continue
		}
		entries = append(entries, entry.Entry{
			Type: "hex:" + name, Path: dir, Name: "mix.exs", Version: ver,
		})
	}
	return entries, nil
}

func looksLikeVersion(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	// A leading 'v' only counts as a version prefix when a digit follows
	// (`v1.2.3`); otherwise plain words like "verbose" would be misread.
	if c == 'v' {
		return len(s) >= 2 && s[1] >= '0' && s[1] <= '9'
	}
	return c == '~' || c == '>' || c == '<' || c == '=' || c == '^' || (c >= '0' && c <= '9')
}

// MixLock parses Elixir's mix.lock. Each line looks like:
//
//	"phoenix": {:hex, :phoenix, "1.7.10", "...", ...},
//
// We pick the name from the lhs key and the version from the third element
// of the tuple (string literal in quotes).
type MixLock struct{}

func (MixLock) Name() string { return "mix-lock" }

func (MixLock) Match(path string) bool { return filepath.Base(path) == "mix.lock" }

var mixLockLineRe = regexp.MustCompile(`"([^"]+)"\s*:\s*\{\s*:hex\s*,\s*:[^,]+,\s*"([^"]+)"`)

func (MixLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for _, m := range mixLockLineRe.FindAllStringSubmatch(string(data), -1) {
		name, ver := m[1], m[2]
		entries = append(entries, entry.Entry{
			Type: "hex:" + name, Path: dir, Name: "mix.lock", Version: ver,
		})
	}
	return entries, nil
}
