package manager

import (
	"path/filepath"
	"testing"
)

func TestUvLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "uv.lock"), `
version = 1

[[package]]
name = "requests"
version = "2.31.0"

[[package]]
name = "urllib3"
version = "2.0.7"
`)
	es := collect(t, UvLock{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["pip:requests"] != "2.31.0" || got["pip:urllib3"] != "2.0.7" {
		t.Errorf("got %v", got)
	}
}
