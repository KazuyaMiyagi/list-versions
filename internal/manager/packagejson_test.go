package manager

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestPackageJSON_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "engines": { "node": " ^20 " },
  "packageManager": " pnpm@9.0.0 "
}`)
	es := collect(t, PackageJSON{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["Node.js"] != "^20" {
		t.Errorf("node engine: want %q, got %q", "^20", got["Node.js"])
	}
	if got["pnpm"] != "9.0.0" {
		t.Errorf("packageManager: want %q, got %q", "9.0.0", got["pnpm"])
	}
}

func TestPackageJSON_Engines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "x",
  "engines": {
    "node": "^20",
    "npm": ">=10",
    "pnpm": ">=9",
    "yarn": "4.x",
    "bun": "1.x",
    "vscode": "^1.80"
  }
}`)
	es := collect(t, PackageJSON{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"Node.js": "^20",
		"npm":     ">=10",
		"pnpm":    ">=9",
		"Yarn":    "4.x",
		"Bun":     "1.x",
	}
	if len(got) != len(want) {
		t.Errorf("got %d entries (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestPackageJSON_PackageManager(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantType    string
		wantVersion string
	}{
		{"pnpm", `{"packageManager": "pnpm@9.0.0"}`, "pnpm", "9.0.0"},
		{"yarn berry", `{"packageManager": "yarn@4.5.0"}`, "Yarn", "4.5.0"},
		{"npm", `{"packageManager": "npm@10.5.0"}`, "npm", "10.5.0"},
		{"with sha", `{"packageManager": "pnpm@9.0.0+sha512.abc"}`, "pnpm", "9.0.0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "package.json"), c.body)
			es := collect(t, PackageJSON{}, dir)
			if len(es) != 1 || es[0].Type != c.wantType || es[0].Version != c.wantVersion {
				t.Errorf("got %+v, want type=%q version=%q", es, c.wantType, c.wantVersion)
			}
		})
	}
}

func TestPackageJSON_NoEngines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"x"}`)
	es := collect(t, PackageJSON{}, dir)
	if len(es) != 0 {
		t.Errorf("expected no entries, got %+v", es)
	}
}

func TestPackageJSON_InvalidJSON(t *testing.T) {
	// A package.json was explicitly detected by name; if it fails to parse
	// the user should hear about it (via ScanAll's warning channel), not
	// silently get zero rows from that file.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `not json`)
	var warn bytes.Buffer
	groups, err := ScanAll([]string{dir}, []Manager{PackageJSON{}}, nil, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if warn.Len() == 0 {
		t.Errorf("expected a warning for invalid JSON so the user hears about it, got none")
	}
	if es := groups[0]; len(es) != 0 {
		t.Errorf("expected no entries on parse failure, got %+v", es)
	}
}
