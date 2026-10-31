package manager

import (
	"path/filepath"
	"testing"
)

func TestUvToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "uv.toml"), `required-version = ">=0.5.0"
`)
	es := collect(t, UvToml{}, dir)
	if len(es) != 1 || es[0].Type != "uv" || es[0].Name != "uv.toml" || es[0].Version != ">=0.5.0" {
		t.Errorf("got %+v, want uv >=0.5.0 from uv.toml", es)
	}
}

func TestUvToml_None(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "uv.toml"), `index-url = "https://example.com/simple"
`)
	if es := collect(t, UvToml{}, dir); len(es) != 0 {
		t.Errorf("got %+v, want no entries", es)
	}
}
