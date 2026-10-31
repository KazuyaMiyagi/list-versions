package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// gemfileRubyRe matches a simple `ruby "X.Y.Z"` or `ruby 'X.Y.Z'` directive at
// the start of a line. The bash version intentionally skips more complex forms
// like `ruby file: ".ruby-version"`, so we do the same.
var gemfileRubyRe = regexp.MustCompile(`^\s*ruby\s+["']([^"']+)["']`)

// Gemfile reads the `ruby "X.Y.Z"` directive from a Gemfile.
type Gemfile struct{}

func (Gemfile) Name() string { return "gemfile" }

func (Gemfile) Match(path string) bool { return isGemfile(path) }

func (Gemfile) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		m := gemfileRubyRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		return []entry.Entry{{
			Type:    "Ruby",
			Path:    filepath.Dir(file.Path),
			Name:    filepath.Base(file.Path),
			Version: m[1],
		}}, nil
	}
	return nil, scanner.Err()
}

// isGemfile also accepts `*.gemfile`, the naming Appraisal uses for the
// per-variant Gemfiles under gemfiles/. The directory is not required, since
// BUNDLE_GEMFILE can point at such a file anywhere.
func isGemfile(path string) bool {
	switch base := filepath.Base(path); base {
	case "Gemfile", "gems.rb":
		return true
	default:
		return strings.HasSuffix(base, ".gemfile")
	}
}

func isGemfileLock(path string) bool {
	switch base := filepath.Base(path); base {
	case "Gemfile.lock", "gems.locked":
		return true
	default:
		return strings.HasSuffix(base, ".gemfile.lock")
	}
}
