package manager

import (
	"path/filepath"
	"testing"
)

func TestGemfileLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile.lock"), `GEM
  remote: https://rubygems.org/
  specs:
    rake (13.0.6)

PLATFORMS
  ruby

DEPENDENCIES
  rake

BUNDLED WITH
   2.5.6
`)
	files := matched(t, GemfileLock{}, dir)
	if len(files) != 1 {
		t.Fatalf("detect: %v", files)
	}
	es := collect(t, GemfileLock{}, dir)
	if len(es) != 1 || es[0].Type != "Bundler" || es[0].Version != "2.5.6" {
		t.Errorf("got %+v", es)
	}
}

func TestGemfileLock_NoBundledWith(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile.lock"), "DEPENDENCIES\n  rake\n")
	es := collect(t, GemfileLock{}, dir)
	if len(es) != 0 {
		t.Errorf("expected no entries, got %+v", es)
	}
}
