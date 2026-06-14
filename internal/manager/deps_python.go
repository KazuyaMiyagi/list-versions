package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// PyprojectTomlDeps extracts the direct dependency list from `pyproject.toml`,
// covering both PEP 621 (`[project] dependencies`, `[project.optional-deps]`)
// and Poetry (`[tool.poetry.dependencies]`).
type PyprojectTomlDeps struct{}

func (PyprojectTomlDeps) Name() string { return "pyproject-toml-deps" }

func (PyprojectTomlDeps) Match(path string) bool { return filepath.Base(path) == "pyproject.toml" }

type pyprojectDepsDoc struct {
	Project struct {
		Dependencies         []string            `toml:"dependencies"`
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
	} `toml:"project"`
	Tool struct {
		Poetry struct {
			Dependencies    map[string]toml.Primitive `toml:"dependencies"`
			DevDependencies map[string]toml.Primitive `toml:"dev-dependencies"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

// pep508NameRe captures the distribution name at the head of a PEP 508
// requirement string ("requests>=2.0", "django[bcrypt]==5.0", etc.).
var pep508NameRe = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-]+)\s*(?:\[[^\]]+\])?\s*(.*)$`)

func parsePEP508(line string) (name, version string) {
	trimmed := strings.TrimSpace(line)
	// URL-style requirements ("git+https://...", "https://...tar.gz",
	// "file:///...") don't carry a distribution name in the usual position;
	// pip identifies them by an explicit `#egg=name` fragment, which we
	// don't pretend to support yet. Drop them rather than emit a bogus
	// `pip:git` / `pip:https` row.
	if strings.HasPrefix(trimmed, "git+") || strings.HasPrefix(trimmed, "hg+") ||
		strings.HasPrefix(trimmed, "svn+") || strings.HasPrefix(trimmed, "bzr+") ||
		strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "file://") {
		return "", ""
	}
	m := pep508NameRe.FindStringSubmatch(line)
	if m == nil {
		return "", ""
	}
	name = m[1]
	rest := strings.TrimSpace(m[2])
	// Drop environment markers (`; python_version >= '3.10'`).
	if i := strings.Index(rest, ";"); i >= 0 {
		rest = strings.TrimSpace(rest[:i])
	}
	return name, rest
}

func (PyprojectTomlDeps) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc pyprojectDepsDoc
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

	for _, line := range doc.Project.Dependencies {
		n, v := parsePEP508(line)
		emit(n, v, "pyproject.toml")
	}
	for groupName, group := range doc.Project.OptionalDependencies {
		nameCol := "pyproject.toml (" + groupName + ")"
		for _, line := range group {
			n, v := parsePEP508(line)
			emit(n, v, nameCol)
		}
	}
	decodePoetryDeps(md, doc.Tool.Poetry.Dependencies, "pyproject.toml", emit)
	decodePoetryDeps(md, doc.Tool.Poetry.DevDependencies, "pyproject.toml (dev)", emit)
	return entries, nil
}

// decodePoetryDeps handles Poetry's two value shapes: a bare string ("^3.13")
// or an inline table ({ version = "^3.13", ... }). `python` is excluded since
// it's a runtime constraint, not a regular dependency.
func decodePoetryDeps(md toml.MetaData, deps map[string]toml.Primitive, nameCol string, emit func(name, ver, nameCol string)) {
	for name, prim := range deps {
		if name == "python" {
			continue
		}
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

// PoetryLock reads `poetry.lock` and emits one entry per resolved package.
type PoetryLock struct{}

func (PoetryLock) Name() string { return "poetry-lock" }

func (PoetryLock) Match(path string) bool { return filepath.Base(path) == "poetry.lock" }

type poetryLockPkg struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type poetryLockDoc struct {
	Package []poetryLockPkg `toml:"package"`
}

func (PoetryLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc poetryLockDoc
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
			Type: "pip:" + p.Name, Path: dir, Name: "poetry.lock", Version: p.Version,
		})
	}
	return entries, nil
}

// RequirementsTxt handles pip's flat requirements file. Lines starting with
// `-r` / `-c` / `--` / `#` are ignored.
type RequirementsTxt struct{}

func (RequirementsTxt) Name() string { return "requirements-txt" }

func (RequirementsTxt) Match(path string) bool {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, ".txt") {
		return false
	}
	// Common pip-tools / pipenv layouts:
	//   requirements.txt
	//   requirements-dev.txt / requirements-test.txt
	//   dev-requirements.txt / test-requirements.txt / prod-requirements.txt
	if name == "requirements.txt" {
		return true
	}
	return strings.HasPrefix(name, "requirements-") || strings.HasSuffix(name, "-requirements.txt")
}

func (RequirementsTxt) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	base := filepath.Base(file.Path)
	var entries []entry.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		// Strip trailing comments.
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		n, v := parsePEP508(line)
		if n == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type: "pip:" + n, Path: dir, Name: base, Version: v,
		})
	}
	return entries, scanner.Err()
}
