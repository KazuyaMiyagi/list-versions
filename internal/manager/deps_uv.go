package manager

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// UvLock reads uv's lockfile (TOML, array of `[[package]]` tables).
type UvLock struct{}

func (UvLock) Name() string { return "uv-lock" }

func (UvLock) Match(path string) bool { return filepath.Base(path) == "uv.lock" }

type uvLockPkg struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type uvLockDoc struct {
	Package []uvLockPkg `toml:"package"`
}

func (UvLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc uvLockDoc
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for _, p := range doc.Package {
		if p.Name == "" || p.Version == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type: "pip:" + p.Name, Path: dir, Name: "uv.lock", Version: p.Version,
		})
	}
	return entries, nil
}
