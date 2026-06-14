package manager

import (
	"encoding/json"
	"path/filepath"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// ComposerJSON reads composer.json's `require` / `require-dev` and emits
// one entry per dependency. `php` and `ext-*` keys are surfaced too because
// they represent PHP runtime / extension constraints the project declares.
type ComposerJSON struct{}

func (ComposerJSON) Name() string { return "composer-json" }

func (ComposerJSON) Match(path string) bool { return filepath.Base(path) == "composer.json" }

type composerJSONDoc struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

func (ComposerJSON) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc composerJSONDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	emit := func(deps map[string]string, nameCol string) {
		for name, ver := range deps {
			entries = append(entries, entry.Entry{
				Type: "composer:" + name, Path: dir, Name: nameCol, Version: ver,
			})
		}
	}
	emit(doc.Require, "composer.json")
	emit(doc.RequireDev, "composer.json (dev)")
	return entries, nil
}

// ComposerLock parses composer.lock; the `packages` and `packages-dev` arrays
// each carry `{ name, version }` entries with the resolved versions.
type ComposerLock struct{}

func (ComposerLock) Name() string { return "composer-lock" }

func (ComposerLock) Match(path string) bool { return filepath.Base(path) == "composer.lock" }

type composerLockPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type composerLockDoc struct {
	Packages    []composerLockPkg `json:"packages"`
	PackagesDev []composerLockPkg `json:"packages-dev"`
}

func (ComposerLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc composerLockDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	emit := func(pkgs []composerLockPkg, nameCol string) {
		for _, p := range pkgs {
			if p.Name == "" || p.Version == "" {
				continue
			}
			entries = append(entries, entry.Entry{
				Type: "composer:" + p.Name, Path: dir, Name: nameCol, Version: p.Version,
			})
		}
	}
	emit(doc.Packages, "composer.lock")
	emit(doc.PackagesDev, "composer.lock (dev)")
	return entries, nil
}
