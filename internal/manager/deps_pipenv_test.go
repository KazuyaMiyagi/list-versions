package manager

import (
	"path/filepath"
	"testing"
)

func TestPipfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Pipfile"), `
[[source]]
url = "https://pypi.org/simple"
verify_ssl = true

[packages]
requests = "*"
django = "==5.0"
flask = { version = ">=3.0" }

[dev-packages]
pytest = "==7.0"
`)
	es := collect(t, Pipfile{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"pip:requests": "",
		"pip:django":   "==5.0",
		"pip:flask":    ">=3.0",
		"pip:pytest":   "==7.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["pip:requests"] != "Pipfile" {
		t.Errorf("prod dep Name: got %q", gotName["pip:requests"])
	}
	if gotName["pip:pytest"] != "Pipfile (dev)" {
		t.Errorf("dev dep Name: got %q", gotName["pip:pytest"])
	}
}

func TestPipfileLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Pipfile.lock"), `{
  "_meta": {},
  "default": {
    "requests": {"version": "==2.31.0"},
    "django": {"version": "==5.0.1"}
  },
  "develop": {
    "pytest": {"version": "==7.0.1"}
  }
}`)
	es := collect(t, PipfileLock{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"pip:requests": "2.31.0",
		"pip:django":   "5.0.1",
		"pip:pytest":   "7.0.1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["pip:requests"] != "Pipfile.lock" {
		t.Errorf("prod dep Name: got %q", gotName["pip:requests"])
	}
	if gotName["pip:pytest"] != "Pipfile.lock (dev)" {
		t.Errorf("dev dep Name: got %q", gotName["pip:pytest"])
	}
}
