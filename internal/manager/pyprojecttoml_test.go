package manager

import (
	"path/filepath"
	"testing"
)

func TestPyprojectToml_PEP621(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[project]
name = "x"
requires-python = ">=3.11"
`)
	es := collect(t, PyprojectToml{}, dir)
	if len(es) != 1 || es[0].Type != "Python" || es[0].Version != ">=3.11" {
		t.Errorf("got %+v", es)
	}
}

func TestPyprojectToml_PoetryString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[tool.poetry.dependencies]
python = "^3.13"
requests = "^2.0"
`)
	es := collect(t, PyprojectToml{}, dir)
	if len(es) != 1 || es[0].Type != "Python" || es[0].Version != "^3.13" {
		t.Errorf("got %+v", es)
	}
}

func TestPyprojectToml_PoetryInlineTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[tool.poetry.dependencies]
python = { version = "^3.12" }
`)
	es := collect(t, PyprojectToml{}, dir)
	if len(es) != 1 || es[0].Version != "^3.12" {
		t.Errorf("got %+v", es)
	}
}

func TestPyprojectToml_PEP621WinsOverPoetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[project]
requires-python = ">=3.11"

[tool.poetry.dependencies]
python = "^3.10"
`)
	es := collect(t, PyprojectToml{}, dir)
	if len(es) != 1 || es[0].Version != ">=3.11" {
		t.Errorf("got %+v, want >=3.11", es)
	}
}

func TestPyprojectToml_None(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[project]
name = "x"
`)
	es := collect(t, PyprojectToml{}, dir)
	if len(es) != 0 {
		t.Errorf("got %+v", es)
	}
}
