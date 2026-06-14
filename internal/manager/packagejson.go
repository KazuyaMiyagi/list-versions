package manager

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// PackageJSON reads package.json and surfaces the runtime/package-manager
// versions declared in `engines` and the Corepack `packageManager` field.
type PackageJSON struct{}

func (PackageJSON) Name() string { return "package-json" }

func (PackageJSON) Match(path string) bool { return filepath.Base(path) == "package.json" }

type packageJSONDoc struct {
	Engines        map[string]string `json:"engines"`
	PackageManager string            `json:"packageManager"`
}

// engineType maps each `engines` key to the TYPE we want to label it as.
// Anything not listed here (e.g. a custom `vscode` engine) is ignored.
var engineType = map[string]string{
	"node": "Node.js",
	"npm":  "npm",
	"pnpm": "pnpm",
	"yarn": "Yarn",
	"bun":  "Bun",
}

func (PackageJSON) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc packageJSONDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	for k, v := range doc.Engines {
		v = strings.TrimSpace(v)
		t, ok := engineType[k]
		if !ok || v == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    t,
			Path:    dir,
			Name:    "package.json",
			Version: v,
		})
	}
	if pm := strings.TrimSpace(doc.PackageManager); pm != "" {
		// Corepack convention: `"pnpm@9.0.0"`, `"yarn@4.5.0"`, `"npm@10.5.0"`,
		// also accepts an optional `+sha512.<hash>` suffix.
		name, version := splitPackageManager(pm)
		if t, ok := engineType[name]; ok && version != "" {
			entries = append(entries, entry.Entry{
				Type:    t,
				Path:    dir,
				Name:    "package.json",
				Version: version,
			})
		}
	}
	return entries, nil
}

func splitPackageManager(s string) (name, version string) {
	at := strings.IndexByte(s, '@')
	if at < 0 {
		return s, ""
	}
	name = s[:at]
	version = s[at+1:]
	if plus := strings.IndexByte(version, '+'); plus >= 0 {
		version = version[:plus]
	}
	return name, version
}
