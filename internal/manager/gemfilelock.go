package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// GemfileLock extracts the `BUNDLED WITH` line that Bundler appends to the
// end of Gemfile.lock. It records which Bundler version produced the lock.
type GemfileLock struct{}

func (GemfileLock) Name() string { return "gemfile-lock" }

func (GemfileLock) Match(path string) bool { return filepath.Base(path) == "Gemfile.lock" }

func (GemfileLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "BUNDLED WITH" {
			continue
		}
		if !scanner.Scan() {
			break
		}
		version := strings.TrimSpace(scanner.Text())
		if version == "" {
			break
		}
		return []entry.Entry{{
			Type:    "Bundler",
			Path:    filepath.Dir(file.Path),
			Name:    "Gemfile.lock",
			Version: version,
		}}, nil
	}
	return nil, scanner.Err()
}
