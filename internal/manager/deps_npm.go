package manager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// PackageJSONDeps emits one entry per dependency declared in package.json
// `dependencies` / `devDependencies`. The version string is the constraint
// the developer wrote (e.g. `^18.2.0`); see PackageLockJSON / YarnLock /
// PnpmLock for resolved versions.
type PackageJSONDeps struct{}

func (PackageJSONDeps) Name() string { return "package-json-deps" }

func (PackageJSONDeps) Match(path string) bool { return filepath.Base(path) == "package.json" }

type packageJSONDeps struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (PackageJSONDeps) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc packageJSONDeps
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for name, ver := range doc.Dependencies {
		entries = append(entries, entry.Entry{
			Type: "npm:" + name, Path: dir, Name: "package.json", Version: ver,
		})
	}
	for name, ver := range doc.DevDependencies {
		entries = append(entries, entry.Entry{
			Type: "npm:" + name, Path: dir, Name: "package.json (dev)", Version: ver,
		})
	}
	return entries, nil
}

// PackageLockJSON reads npm's package-lock.json and emits one entry per
// resolved package — direct and transitive alike, matching the other lockfile
// managers (Cargo.lock, poetry.lock, Gemfile.lock). The full installed set is
// what's useful for spotting a vulnerable pinned version anywhere in the tree.
type PackageLockJSON struct{}

func (PackageLockJSON) Name() string { return "package-lock-json" }

func (PackageLockJSON) Match(path string) bool { return filepath.Base(path) == "package-lock.json" }

type packageLockDoc struct {
	LockfileVersion int `json:"lockfileVersion"`
	// v2/v3: keyed by path under node_modules; the project root has key "".
	Packages map[string]struct {
		Version string `json:"version"`
		Link    bool   `json:"link"`
		Dev     bool   `json:"dev"`
	} `json:"packages"`
	// v1: top-level resolved tree (hoisted, so direct + transitive).
	Dependencies map[string]struct {
		Version string `json:"version"`
		Dev     bool   `json:"dev"`
	} `json:"dependencies"`
}

func (PackageLockJSON) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc packageLockDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	nameCol := func(dev bool) string {
		if dev {
			return "package-lock.json (dev)"
		}
		return "package-lock.json"
	}
	var entries []entry.Entry
	// v2/v3 lockfiles carry both "packages" and "dependencies"; prefer
	// "packages", whose keys enumerate every installed package by its path
	// under node_modules ("" is the project itself).
	if len(doc.Packages) > 0 {
		for key, pkg := range doc.Packages {
			// Skip the project root and workspace symlinks (no real version).
			if key == "" || pkg.Link || pkg.Version == "" {
				continue
			}
			name := packageNameFromLockKey(key)
			if name == "" {
				continue
			}
			entries = append(entries, entry.Entry{
				Type: "npm:" + name, Path: dir, Name: nameCol(pkg.Dev), Version: pkg.Version,
			})
		}
		return entries, nil
	}
	// v1: the hoisted top-level tree.
	for name, p := range doc.Dependencies {
		entries = append(entries, entry.Entry{
			Type: "npm:" + name, Path: dir, Name: nameCol(p.Dev), Version: p.Version,
		})
	}
	return entries, nil
}

// packageNameFromLockKey turns a package-lock.json v2/v3 "packages" key (a path
// under node_modules, possibly nested) into the bare package name:
//
//	node_modules/react                   -> react
//	node_modules/@types/node             -> @types/node
//	node_modules/foo/node_modules/@s/bar -> @s/bar
func packageNameFromLockKey(key string) string {
	const marker = "node_modules/"
	idx := strings.LastIndex(key, marker)
	if idx < 0 {
		return ""
	}
	return key[idx+len(marker):]
}

// YarnLock parses Yarn lockfiles. A block looks like:
//
//	"react@^18.2.0":          # v1            "react@npm:^18.2.0":      # Berry (v2+)
//	  version "18.3.1"                          version: 18.3.1
//
// We split the comma-separated key on `@<version>` to recover the name (which
// may itself start with `@scope/`) and pair it with the `version` line. Both
// the v1 (`version "x"`) and Berry (`version: x`, optionally quoted) shapes are
// supported.
type YarnLock struct{}

func (YarnLock) Name() string { return "yarn-lock" }

func (YarnLock) Match(path string) bool { return filepath.Base(path) == "yarn.lock" }

// yarnVersionRe matches both `version "18.3.1"` (v1) and `version: 18.3.1` /
// `version: "18.3.1"` (Berry). The value runs to the first quote or whitespace,
// which is safe because versions never contain either.
var yarnVersionRe = regexp.MustCompile(`^\s+version:?\s+"?([^"\s]+)"?\s*$`)

func (YarnLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry

	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Yarn lockfile blocks can contain very long key lines; bump the buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var pendingNames []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			// Header line: one or more comma-separated `name@spec` keys ending in `:`.
			pendingNames = parseYarnHeader(line)
			continue
		}
		if m := yarnVersionRe.FindStringSubmatch(line); m != nil {
			ver := m[1]
			for _, name := range pendingNames {
				entries = append(entries, entry.Entry{
					Type: "npm:" + name, Path: dir, Name: "yarn.lock", Version: ver,
				})
			}
			pendingNames = nil
		}
	}
	return entries, scanner.Err()
}

// parseYarnHeader splits the comma-separated keys in a yarn.lock block header
// into the names they refer to. The trailing `:` is stripped, each key is
// optionally quoted, and the spec after the **last** `@` is the version
// constraint (kept off so we only return the package name).
func parseYarnHeader(line string) []string {
	line = strings.TrimSuffix(strings.TrimSpace(line), ":")
	var names []string
	for _, raw := range strings.Split(line, ",") {
		k := strings.TrimSpace(raw)
		k = strings.Trim(k, "\"")
		// URL-based specs (`name@git+ssh://git@host/...`) contain extra
		// `@` characters inside the URL; LastIndex('@') splits them in the
		// middle of the URL and yields garbage. Skip them entirely.
		if strings.Contains(k, "://") {
			continue
		}
		// Strip the spec after the last '@'. For scoped packages the name
		// starts with '@', so begin the search after index 0.
		searchFrom := 0
		if strings.HasPrefix(k, "@") {
			searchFrom = 1
		}
		at := strings.LastIndex(k[searchFrom:], "@")
		if at < 0 {
			continue
		}
		name := k[:searchFrom+at]
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	// Dedupe while preserving order.
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// PnpmLock parses pnpm-lock.yaml. The relevant top-level keys are
// `importers.<workspace>.dependencies` / `devDependencies`, each of which is
// a map of name -> { specifier, version }. v5 lockfiles store deps under
// `dependencies` at the root instead of inside `importers`.
type PnpmLock struct{}

func (PnpmLock) Name() string { return "pnpm-lock" }

func (PnpmLock) Match(path string) bool { return filepath.Base(path) == "pnpm-lock.yaml" }

type pnpmLockEntry struct {
	Specifier string `yaml:"specifier"`
	Version   string `yaml:"version"`
}

type pnpmLockDoc struct {
	Dependencies    map[string]pnpmLockEntry `yaml:"dependencies"`
	DevDependencies map[string]pnpmLockEntry `yaml:"devDependencies"`
	Importers       map[string]struct {
		Dependencies    map[string]pnpmLockEntry `yaml:"dependencies"`
		DevDependencies map[string]pnpmLockEntry `yaml:"devDependencies"`
	} `yaml:"importers"`
}

func (PnpmLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc pnpmLockDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	add := func(deps map[string]pnpmLockEntry, nameCol string) {
		for name, e := range deps {
			v := e.Version
			// pnpm stores transitive paths like `1.2.3(react@18.3.1)`; strip
			// everything after the first `(`.
			if i := strings.IndexByte(v, '('); i >= 0 {
				v = v[:i]
			}
			if v == "" {
				continue
			}
			entries = append(entries, entry.Entry{
				Type: "npm:" + name, Path: dir, Name: nameCol, Version: v,
			})
		}
	}
	add(doc.Dependencies, "pnpm-lock.yaml")
	add(doc.DevDependencies, "pnpm-lock.yaml (dev)")
	for _, imp := range doc.Importers {
		add(imp.Dependencies, "pnpm-lock.yaml")
		add(imp.DevDependencies, "pnpm-lock.yaml (dev)")
	}
	return entries, nil
}

// BunLock parses bun's text lockfile (`bun.lock`, JSONC-like). The binary
// counterpart `bun.lockb` is intentionally not supported because it requires
// running `bun` to decode.
type BunLock struct{}

func (BunLock) Name() string { return "bun-lock" }

func (BunLock) Match(path string) bool { return filepath.Base(path) == "bun.lock" }

// Bun's text lockfile is JSONC (allows comments and trailing commas). We
// don't decode the whole document — we only care about the contents of the
// top-level `"packages": { ... }` table, scoped via a brace counter.
var bunPackageKeyRe = regexp.MustCompile(`"([^"\s][^"]*)"\s*:\s*\[\s*"([^"]+)"`)

func (BunLock) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	src := string(data)
	packagesBlock := extractBunPackagesBlock(src)
	if packagesBlock == "" {
		return nil, nil
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for _, m := range bunPackageKeyRe.FindAllStringSubmatch(packagesBlock, -1) {
		name := m[1]
		spec := m[2]
		// URL-based specs (`name@git+ssh://git@host/...`, `https://...`)
		// can't be split by LastIndex('@') because the URL itself
		// contains `@`. Skip them — we have no clean version to report.
		if strings.Contains(spec, "://") {
			continue
		}
		// spec is typically "<name>@<version>" — pull the version off the
		// last `@` (after the optional scope prefix).
		searchFrom := 0
		if strings.HasPrefix(spec, "@") {
			searchFrom = 1
		}
		at := strings.LastIndex(spec[searchFrom:], "@")
		if at < 0 {
			continue
		}
		ver := spec[searchFrom+at+1:]
		entries = append(entries, entry.Entry{
			Type: "npm:" + name, Path: dir, Name: "bun.lock", Version: ver,
		})
	}
	return entries, nil
}

// extractBunPackagesBlock isolates the content of the top-level `"packages":
// { ... }` object so later regex scans don't accidentally match `"key": ["v"`
// patterns living in sibling tables like `workspaces.*.dependencies`.
func extractBunPackagesBlock(src string) string {
	idx := strings.Index(src, `"packages"`)
	if idx < 0 {
		return ""
	}
	rest := src[idx:]
	open := strings.IndexByte(rest, '{')
	if open < 0 {
		return ""
	}
	depth := 0
	for i := open; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[open : i+1]
			}
		}
	}
	return ""
}
