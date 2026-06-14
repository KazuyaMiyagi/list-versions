package manager

import (
	"path/filepath"
	"testing"
)

func TestHelm(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "charts/myapp/Chart.yaml"), `
apiVersion: v2
name: myapp
version: 1.2.3
appVersion: "4.5.6"
`)
	writeFile(t, filepath.Join(dir, "charts/myapp/values.yaml"), `
image:
  repository: myorg/myapp
  tag: "1.0.0"
`)
	files := matched(t, Helm{}, dir)
	if len(files) != 1 {
		t.Fatalf("detect: %v", files)
	}
	es := collect(t, Helm{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Name] = e.Version
	}
	want := map[string]string{
		"Chart.yaml:appVersion": "4.5.6",
		"Chart.yaml:version":    "1.2.3",
		"values.yaml:image.tag": "myorg/myapp:1.0.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestHelm_NoValuesFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), `
name: foo
version: 0.1.0
appVersion: "1.0"
`)
	es := collect(t, Helm{}, dir)
	if len(es) != 2 {
		t.Errorf("expected 2 entries (no values.yaml), got %+v", es)
	}
}
