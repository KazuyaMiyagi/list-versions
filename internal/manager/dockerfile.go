package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// fromRe matches `FROM [--platform=...] <image>[:<tag>][@<digest>] [AS <stage>]`.
var fromRe = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)`)

// argRe matches `ARG NAME[=default]` (default value may be quoted).
var argRe = regexp.MustCompile(`(?i)^\s*ARG\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s*=\s*(.+?))?\s*$`)

// argRefRe finds `$NAME` / `${NAME}` references inside a FROM target.
var argRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

type Dockerfile struct{}

func (Dockerfile) Name() string { return "dockerfile" }

func (Dockerfile) Match(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	return lower == "dockerfile" || strings.HasPrefix(lower, "dockerfile.")
}

func (Dockerfile) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}

	base := filepath.Base(file.Path)
	dir := filepath.Dir(file.Path)
	seen := map[string]bool{}
	args := map[string]string{}
	var entries []entry.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if m := argRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			def := strings.Trim(strings.TrimSpace(m[2]), `"'`)
			if def != "" {
				args[name] = def
			}
			continue
		}
		m := fromRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ref, resolved := substituteArgs(m[1], args)
		// If interpolation tokens remain unresolved, drop the line.
		if !resolved {
			continue
		}
		// Require either a tag or a digest so we have a real version to
		// report. Tag is at the last ':' inside the final path segment
		// (so registry hosts like "host:5000/foo:1.0" still parse) and a
		// digest sits after '@'.
		core := ref
		if i := strings.Index(core, "@"); i >= 0 {
			core = core[:i]
		}
		slash := strings.LastIndexByte(core, '/')
		last := core
		if slash >= 0 {
			last = core[slash+1:]
		}
		hasTag := strings.IndexByte(last, ':') >= 0
		hasDigest := strings.Contains(ref, "@")
		if !hasTag && !hasDigest {
			continue
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		entries = append(entries, entry.Entry{
			Type:    "Docker",
			Path:    dir,
			Name:    base,
			Version: ref,
		})
	}
	return entries, scanner.Err()
}

// substituteArgs replaces $NAME / ${NAME} tokens with their ARG default. It
// returns the substituted string and false if any token had no known default
// (so the caller can skip a FROM whose image is dynamic at build time).
func substituteArgs(s string, args map[string]string) (string, bool) {
	resolved := true
	out := argRefRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := argRefRe.FindStringSubmatch(match)
		name := sub[1]
		if name == "" {
			name = sub[2]
		}
		if v, ok := args[name]; ok {
			return v
		}
		resolved = false
		return match
	})
	return out, resolved
}
