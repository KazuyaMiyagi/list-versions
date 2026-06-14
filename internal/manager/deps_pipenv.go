package manager

import (
	"encoding/json"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// Pipfile reads pipenv's Pipfile (TOML) and emits one entry per package in
// `[packages]` / `[dev-packages]`. Each value can be a bare version string
// (`"*"`, `"==1.0"`) or an inline table.
type Pipfile struct{}

func (Pipfile) Name() string { return "pipfile" }

func (Pipfile) Match(path string) bool { return filepath.Base(path) == "Pipfile" }

type pipfileDoc struct {
	Packages    map[string]toml.Primitive `toml:"packages"`
	DevPackages map[string]toml.Primitive `toml:"dev-packages"`
}

func (Pipfile) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc pipfileDoc
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	emit := func(name, ver, nameCol string) {
		if name == "" {
			return
		}
		entries = append(entries, entry.Entry{
			Type: "pip:" + name, Path: dir, Name: nameCol, Version: ver,
		})
	}
	walk := func(deps map[string]toml.Primitive, nameCol string) {
		for name, prim := range deps {
			var s string
			if err := md.PrimitiveDecode(prim, &s); err == nil {
				if s == "*" {
					s = ""
				}
				emit(name, s, nameCol)
				continue
			}
			var inline struct {
				Version string `toml:"version"`
			}
			if err := md.PrimitiveDecode(prim, &inline); err == nil {
				emit(name, inline.Version, nameCol)
			}
		}
	}
	walk(doc.Packages, "Pipfile")
	walk(doc.DevPackages, "Pipfile (dev)")
	return entries, nil
}

// PipfileLock reads Pipfile.lock (JSON) and emits resolved versions from the
// `default` and `develop` sections. The value for each name has a "version"
// field like `"==2.31.0"`; we strip the leading `==`.
type PipfileLock struct{}

func (PipfileLock) Name() string { return "pipfile-lock" }

func (PipfileLock) Match(path string) bool { return filepath.Base(path) == "Pipfile.lock" }

type pipfileLockEntry struct {
	Version string `json:"version"`
}

type pipfileLockDoc struct {
	Default map[string]pipfileLockEntry `json:"default"`
	Develop map[string]pipfileLockEntry `json:"develop"`
}

func (PipfileLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc pipfileLockDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	add := func(deps map[string]pipfileLockEntry, nameCol string) {
		for name, e := range deps {
			v := e.Version
			if len(v) >= 2 && v[:2] == "==" {
				v = v[2:]
			}
			if v == "" {
				continue
			}
			entries = append(entries, entry.Entry{
				Type: "pip:" + name, Path: dir, Name: nameCol, Version: v,
			})
		}
	}
	add(doc.Default, "Pipfile.lock")
	add(doc.Develop, "Pipfile.lock (dev)")
	return entries, nil
}
