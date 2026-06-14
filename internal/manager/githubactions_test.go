package manager

import (
	"path/filepath"
	"sort"
	"testing"
)

func TestGithubActions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - uses: ruby/setup-ruby@v1
        with:
          ruby-version: 3.3
      - uses: actions/cache@abcdef1234567890abcdef1234567890abcdef12 # v4.2.0
      - uses: pnpm/action-setup@v3
        with:
          version: 9.0.0
      - uses: actions/setup-node@v4
        with:
          node-version-file: .nvmrc
      - uses: oven-sh/setup-bun@v2
        with:
          bun-version: 1.1.0
      - uses: actions/checkout@v4    # duplicate, dedup
      - uses: ./.github/actions/local # local, skip
      - uses: docker://alpine:3       # docker, skip
      - run: echo no uses
`)

	files := matched(t, GithubActions{}, dir)
	if len(files) != 1 {
		t.Fatalf("want 1 workflow file, got %d", len(files))
	}
	es := collect(t, GithubActions{}, dir)

	type pair struct{ Type, Version string }
	got := map[pair]bool{}
	for _, e := range es {
		if e.Name != "ci.yml" {
			t.Errorf("expected Name=ci.yml, got %q", e.Name)
		}
		got[pair{e.Type, e.Version}] = true
	}
	want := []pair{
		{"GitHub Actions", "actions/checkout@v4"},
		{"GitHub Actions", "actions/setup-node@v4"},
		{"GitHub Actions", "actions/setup-python@v5"},
		{"GitHub Actions", "ruby/setup-ruby@v1"},
		{"GitHub Actions", "actions/cache@abcdef1234567890abcdef1234567890abcdef12 # v4.2.0"},
		{"GitHub Actions", "pnpm/action-setup@v3"},
		{"GitHub Actions", "oven-sh/setup-bun@v2"},
		{"Node.js", "20"},
		{"Node.js", ".nvmrc"},
		{"Python", "3.12"},
		{"Ruby", "3.3"},
		{"pnpm", "9.0.0"},
		{"Bun", "1.1.0"},
	}
	if len(got) != len(want) {
		var list []string
		for p := range got {
			list = append(list, p.Type+"="+p.Version)
		}
		sort.Strings(list)
		t.Errorf("got %d entries (%v), want %d (%v)", len(got), list, len(want), want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %+v", w)
		}
	}
}

func TestGithubActions_EnvResolution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
name: CI
on: push
env:
  RUBY_VERSION: 4.0.5
  NODE_VERSION: '20.10.0'
jobs:
  test:
    runs-on: ubuntu-latest
    env:
      PYTHON_VERSION: '3.12'
    steps:
      - uses: ruby/setup-ruby@v1
        with:
          ruby-version: ${{ env.RUBY_VERSION }}
      - uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
      - uses: actions/setup-python@v5
        with:
          python-version: ${{ env.PYTHON_VERSION }}
      - uses: actions/setup-go@v5
        env:
          GO_VERSION: 1.22
        with:
          go-version: ${{ env.GO_VERSION }}
      - uses: actions/setup-java@v4
        with:
          java-version: ${{ env.MISSING }}
`)
	es := collect(t, GithubActions{}, dir)

	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	wants := map[string]string{
		"Ruby":    "4.0.5",              // workflow env
		"Node.js": "20.10.0",            // workflow env
		"Python":  "3.12",               // job env overrides
		"Go":      "1.22",               // step env overrides
		"Java":    "${{ env.MISSING }}", // unresolved -> kept as-is
	}
	for k, want := range wants {
		if got[k] != want {
			t.Errorf("%s: got %q, want %q", k, got[k], want)
		}
	}
}

func TestGithubActions_MatrixResolution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        node-version: [18, 20, 22]
        python-version: ['3.11', '3.12']
        include:
          - node-version: 24
            python-version: '3.13'
    steps:
      - uses: actions/setup-node@v4
        with:
          node-version: ${{ matrix.node-version }}
      - uses: actions/setup-python@v5
        with:
          python-version: ${{ matrix.python-version }}
`)
	es := collect(t, GithubActions{}, dir)

	nodes := map[string]bool{}
	pythons := map[string]bool{}
	for _, e := range es {
		switch e.Type {
		case "Node.js":
			nodes[e.Version] = true
		case "Python":
			pythons[e.Version] = true
		}
	}
	for _, v := range []string{"18", "20", "22"} {
		if !nodes[v] {
			t.Errorf("missing Node.js %s, got %v", v, nodes)
		}
	}
	for _, v := range []string{"3.11", "3.12"} {
		if !pythons[v] {
			t.Errorf("missing Python %s, got %v", v, pythons)
		}
	}
}

func TestGithubActions_MatrixCrossProduct(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
jobs:
  test:
    strategy:
      matrix:
        major: [1, 2]
        minor: [a, b]
    steps:
      - uses: actions/setup-node@v4
        with:
          node-version: ${{ matrix.major }}.${{ matrix.minor }}
`)
	es := collect(t, GithubActions{}, dir)

	got := map[string]bool{}
	for _, e := range es {
		if e.Type == "Node.js" {
			got[e.Version] = true
		}
	}
	for _, v := range []string{"1.a", "1.b", "2.a", "2.b"} {
		if !got[v] {
			t.Errorf("missing %s, got %v", v, got)
		}
	}
}

func TestGithubActions_DetectIgnoresNonWorkflowDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github/other/ci.yml"), "")
	writeFile(t, filepath.Join(dir, "workflows/ci.yml"), "")
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
jobs:
  x:
    steps:
      - uses: actions/checkout@v4
`)
	files := matched(t, GithubActions{}, dir)
	if len(files) != 1 {
		t.Errorf("want 1 file matched, got %d (%v)", len(files), files)
	}
}
