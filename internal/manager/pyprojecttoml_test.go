package manager

import (
	"path/filepath"
	"reflect"
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

func TestPyprojectToml_RequiresPoetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[project]
requires-python = ">=3.11"

[tool.poetry]
requires-poetry = ">=2.0"
`)
	got := map[string]string{}
	for _, e := range collect(t, PyprojectToml{}, dir) {
		got[e.Type] = e.Version
	}
	want := map[string]string{"Python": ">=3.11", "Poetry": ">=2.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPyprojectToml_UvRequiredVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[project]
requires-python = ">=3.12"

[tool.uv]
required-version = ">=0.5.0"
`)
	got := map[string]string{}
	for _, e := range collect(t, PyprojectToml{}, dir) {
		got[e.Type] = e.Version
	}
	want := map[string]string{"Python": ">=3.12", "uv": ">=0.5.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPyprojectToml_PackageManagerWithoutPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `
[tool.poetry]
requires-poetry = ">=2.0"
`)
	got := map[string]string{}
	for _, e := range collect(t, PyprojectToml{}, dir) {
		got[e.Type] = e.Version
	}
	want := map[string]string{"Poetry": ">=2.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
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
