package manager

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// CargoToml extracts dependencies from `Cargo.toml`'s `[dependencies]`,
// `[dev-dependencies]`, and `[build-dependencies]` tables. Each value can be
// a bare version string or an inline table; we handle both via toml.Primitive.
type CargoToml struct{}

func (CargoToml) Name() string { return "cargo-toml" }

func (CargoToml) Match(path string) bool { return filepath.Base(path) == "Cargo.toml" }

type cargoTomlDoc struct {
	Dependencies      map[string]toml.Primitive `toml:"dependencies"`
	DevDependencies   map[string]toml.Primitive `toml:"dev-dependencies"`
	BuildDependencies map[string]toml.Primitive `toml:"build-dependencies"`
}

func (CargoToml) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc cargoTomlDoc
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
			Type: "cargo:" + name, Path: dir, Name: nameCol, Version: ver,
		})
	}
	walk := func(deps map[string]toml.Primitive, nameCol string) {
		for name, prim := range deps {
			var s string
			if err := md.PrimitiveDecode(prim, &s); err == nil {
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
	walk(doc.Dependencies, "Cargo.toml")
	walk(doc.DevDependencies, "Cargo.toml (dev)")
	walk(doc.BuildDependencies, "Cargo.toml (build)")
	return entries, nil
}

// CargoLock reads `Cargo.lock` and emits one entry per resolved crate.
type CargoLock struct{}

func (CargoLock) Name() string { return "cargo-lock" }

func (CargoLock) Match(path string) bool { return filepath.Base(path) == "Cargo.lock" }

type cargoLockPkg struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type cargoLockDoc struct {
	Package []cargoLockPkg `toml:"package"`
}

func (CargoLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc cargoLockDoc
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
			Type: "cargo:" + p.Name, Path: dir, Name: "Cargo.lock", Version: p.Version,
		})
	}
	return entries, nil
}
