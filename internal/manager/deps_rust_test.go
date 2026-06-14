package manager

import (
	"path/filepath"
	"testing"
)

func TestCargoToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `
[package]
name = "x"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "1.35", features = ["full"] }

[dev-dependencies]
proptest = "1.4"

[build-dependencies]
cc = "1.0"
`)
	es := collect(t, CargoToml{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"cargo:serde":    "1.0",
		"cargo:tokio":    "1.35",
		"cargo:proptest": "1.4",
		"cargo:cc":       "1.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	wantName := map[string]string{
		"cargo:serde":    "Cargo.toml",
		"cargo:proptest": "Cargo.toml (dev)",
		"cargo:cc":       "Cargo.toml (build)",
	}
	for k, v := range wantName {
		if gotName[k] != v {
			t.Errorf("%s Name: got %q, want %q", k, gotName[k], v)
		}
	}
}

func TestCargoLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.lock"), `
[[package]]
name = "serde"
version = "1.0.193"

[[package]]
name = "tokio"
version = "1.35.1"
`)
	es := collect(t, CargoLock{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["cargo:serde"] != "1.0.193" || got["cargo:tokio"] != "1.35.1" {
		t.Errorf("got %v", got)
	}
}
