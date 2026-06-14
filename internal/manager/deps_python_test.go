package manager

import (
	"path/filepath"
	"testing"
)

func TestPyprojectTomlDeps_PEP621(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[project]
name = "x"
dependencies = [
  "requests>=2.0",
  "django[bcrypt]==5.0",
  "rich; python_version>='3.10'",
]

[project.optional-dependencies]
dev = ["pytest>=7.0"]
`)
	es := collect(t, PyprojectTomlDeps{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"pip:requests": ">=2.0",
		"pip:django":   "==5.0",
		"pip:rich":     "",
		"pip:pytest":   ">=7.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["pip:requests"] != "pyproject.toml" {
		t.Errorf("prod dep Name: got %q", gotName["pip:requests"])
	}
	// pytest lives under [project.optional-dependencies] dev -> labelled by group.
	if gotName["pip:pytest"] != "pyproject.toml (dev)" {
		t.Errorf("optional-dependency Name: got %q", gotName["pip:pytest"])
	}
}

func TestPyprojectTomlDeps_Poetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[tool.poetry.dependencies]
python = "^3.11"
requests = "^2.0"
django = { version = "^5.0", extras = ["bcrypt"] }

[tool.poetry.dev-dependencies]
pytest = "^7.0"
`)
	es := collect(t, PyprojectTomlDeps{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"pip:requests": "^2.0",
		"pip:django":   "^5.0",
		"pip:pytest":   "^7.0",
	}
	if _, ok := got["pip:python"]; ok {
		t.Errorf("python should be excluded (runtime constraint), got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["pip:requests"] != "pyproject.toml" {
		t.Errorf("prod dep Name: got %q", gotName["pip:requests"])
	}
	if gotName["pip:pytest"] != "pyproject.toml (dev)" {
		t.Errorf("poetry dev dep Name: got %q", gotName["pip:pytest"])
	}
}

func TestPoetryLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "poetry.lock"), `
[[package]]
name = "requests"
version = "2.31.0"
description = "..."

[[package]]
name = "urllib3"
version = "2.0.7"
`)
	es := collect(t, PoetryLock{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["pip:requests"] != "2.31.0" || got["pip:urllib3"] != "2.0.7" {
		t.Errorf("got %v", got)
	}
}

func TestRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), `# comment
requests>=2.0
django[bcrypt]==5.0
-r requirements-dev.txt
--index-url https://...
flask  # inline comment
git+https://github.com/foo/bar.git@v1.2.3#egg=bar
https://example.com/pkg-1.0.tar.gz
`)
	es := collect(t, RequirementsTxt{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"pip:requests": ">=2.0",
		"pip:django":   "==5.0",
		"pip:flask":    "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	for _, bad := range []string{"pip:git", "pip:https", "pip:bar"} {
		if _, ok := got[bad]; ok {
			t.Errorf("URL-style dep should not produce %s, got %v", bad, got)
		}
	}
}

func TestRequirementsTxt_DetectVariants(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"requirements.txt",
		"requirements-dev.txt",
		"dev-requirements.txt",
		"test-requirements.txt",
		"unrelated.txt",
	} {
		writeFile(t, filepath.Join(dir, name), "requests>=2.0\n")
	}
	files := matched(t, RequirementsTxt{}, dir)
	got := map[string]bool{}
	for _, f := range files {
		got[filepath.Base(f.Path)] = true
	}
	want := []string{
		"requirements.txt",
		"requirements-dev.txt",
		"dev-requirements.txt",
		"test-requirements.txt",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing detection of %s, got %v", w, got)
		}
	}
	if got["unrelated.txt"] {
		t.Errorf("unrelated.txt should not match Detect")
	}
}
