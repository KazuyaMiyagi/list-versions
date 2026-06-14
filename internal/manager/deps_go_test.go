package manager

import (
	"path/filepath"
	"testing"
)

func TestGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module example.com/foo

go 1.22

require (
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
	github.com/davecgh/go-spew v1.1.1 // indirect
)

require github.com/stretchr/testify v1.9.0

require github.com/some/transitive v0.0.0 // indirect
`)
	es := collect(t, GoMod{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"go:github.com/spf13/cobra":      "v1.8.0",
		"go:gopkg.in/yaml.v3":            "v3.0.1",
		"go:github.com/stretchr/testify": "v1.9.0",
	}
	if len(got) != len(want) {
		t.Errorf("got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}
