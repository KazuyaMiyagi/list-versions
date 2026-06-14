package manager

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// matched returns the Files that m claims under root, mirroring the scanner:
// file-scoped managers yield one File per matched file; DirScoped managers
// collapse matches to one File per directory. Used by tests asserting which
// paths a manager picks up.
func matched(t *testing.T, m Manager, root string) []File {
	t.Helper()
	files, err := walkFiles(root, defaultPruneDirs)
	if err != nil {
		t.Fatal(err)
	}
	if dirScoped(m) {
		seen := map[string]bool{}
		var out []File
		for _, p := range files {
			if !m.Match(p) {
				continue
			}
			d := filepath.Dir(p)
			if seen[d] {
				continue
			}
			seen[d] = true
			out = append(out, File{Path: d})
		}
		return out
	}
	var out []File
	for _, p := range files {
		if m.Match(p) {
			out = append(out, File{Path: p})
		}
	}
	return out
}

// collect runs the real scanner for a single manager over root and returns its
// entries as produced (the CLI sorts later; tests sort themselves when needed).
func collect(t *testing.T, m Manager, root string) []entry.Entry {
	t.Helper()
	groups, err := ScanAll([]string{root}, []Manager{m}, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	return groups[0]
}

func TestVersionFile_Extract(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".python-version"), "3.13.13\n")
	writeFile(t, filepath.Join(dir, "sub", ".ruby-version"), "ruby-4.0.5\n")
	writeFile(t, filepath.Join(dir, "node_modules", ".python-version"), "should be skipped\n")
	writeFile(t, filepath.Join(dir, ".terraform", ".terraform-version"), "skip\n")

	files := matched(t, VersionFile{}, dir)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d (%v)", len(files), files)
	}

	entries := collect(t, VersionFile{}, dir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Type != "Python" || entries[0].Version != "3.13.13" {
		t.Errorf("python entry: %+v", entries[0])
	}
	if entries[1].Type != "Ruby" || entries[1].Version != "ruby-4.0.5" {
		t.Errorf("ruby entry: %+v", entries[1])
	}
}

func TestVersionFile_UnknownLabelFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".something-version"), "1.0\n")
	es := collect(t, VersionFile{}, dir)
	if len(es) != 1 || es[0].Type != "something" {
		t.Errorf("fallback type: %+v", es)
	}
}

// A UTF-8 BOM (common on Windows-edited files) must not leak into the version.
func TestVersionFile_StripsBOM(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".python-version"), "\xef\xbb\xbf3.13.13\n")
	es := collect(t, VersionFile{}, dir)
	if len(es) != 1 || es[0].Version != "3.13.13" {
		t.Errorf("want version 3.13.13 with BOM stripped, got %+v", es)
	}
}

func TestNvmrc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".nvmrc"), "20.10.0\n")
	files := matched(t, Nvmrc{}, dir)
	if len(files) != 1 {
		t.Fatalf("want 1, got %d", len(files))
	}
	es := collect(t, Nvmrc{}, dir)
	if len(es) != 1 || es[0].Type != "Node.js" || es[0].Version != "20.10.0" {
		t.Errorf("got %+v", es)
	}
}

func TestToolVersions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".tool-versions"), `# comment
python 3.13.13
ruby 4.0.5 3.4.9 # multiple versions
nodejs 22.0.0
weird-tool 1.0
`)
	es := collect(t, ToolVersions{}, dir)
	if len(es) != 5 {
		t.Fatalf("want 5 entries (1 python, 2 ruby, 1 node, 1 weird), got %d: %+v", len(es), es)
	}
	want := map[string]string{
		"Python":     "3.13.13",
		"Node.js":    "22.0.0",
		"weird-tool": "1.0",
	}
	got := map[string]string{}
	rubyCount := 0
	for _, e := range es {
		if e.Type == "Ruby" {
			rubyCount++
			continue
		}
		got[e.Type] = e.Version
	}
	if rubyCount != 2 {
		t.Errorf("want 2 Ruby entries, got %d", rubyCount)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: want %q, got %q", k, v, got[k])
		}
	}
}

func TestVersionFile_SkipsNvmrcAndToolVersions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".nvmrc"), "20")
	writeFile(t, filepath.Join(dir, ".tool-versions"), "python 3.13.13")
	files := matched(t, VersionFile{}, dir)
	if len(files) != 0 {
		t.Errorf("VersionFile should skip .nvmrc and .tool-versions, got %+v", files)
	}
}
