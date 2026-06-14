package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// VersionFile reads `.<lang>-version` style files (and `.rvmrc`), emitting one
// entry per non-comment line. `.nvmrc` and `.tool-versions` are deliberately
// left to the Nvmrc and ToolVersions managers.
type VersionFile struct{}

func (VersionFile) Name() string { return "version-file" }

func (VersionFile) Match(path string) bool {
	name := filepath.Base(path)
	if name == ".nvmrc" {
		return false // handled by Nvmrc
	}
	if name == ".tool-versions" {
		return false // handled by ToolVersions
	}
	return (strings.HasPrefix(name, ".") && strings.HasSuffix(name, "-version")) ||
		name == ".rvmrc"
}

func (VersionFile) Extract(file File) ([]entry.Entry, error) {
	base := filepath.Base(file.Path)
	typeName := typeFromVersionFile(base)

	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}

	var entries []entry.Entry
	dir := filepath.Dir(file.Path)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    typeName,
			Path:    dir,
			Name:    base,
			Version: line,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Nvmrc reads `.nvmrc`, reporting its single line as a Node.js version.
type Nvmrc struct{}

func (Nvmrc) Name() string { return "nvmrc" }

func (Nvmrc) Match(path string) bool { return filepath.Base(path) == ".nvmrc" }

func (Nvmrc) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}

	var entries []entry.Entry
	dir := filepath.Dir(file.Path)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "Node.js",
			Path:    dir,
			Name:    ".nvmrc",
			Version: line,
		})
	}
	return entries, scanner.Err()
}
