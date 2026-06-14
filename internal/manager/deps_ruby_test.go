package manager

import (
	"path/filepath"
	"testing"
)

func TestGemfileDeps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile"), `
source "https://rubygems.org"

ruby "3.3.0"

gem "rails", "~> 7.0"
gem "sidekiq", ">= 7.1", "< 8"
gem "puma"
# gem "commented-out"
gem "rspec-rails", group: [:development, :test]
`)
	es := collect(t, GemfileDeps{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"gem:rails":       "~> 7.0",
		"gem:sidekiq":     ">= 7.1",
		"gem:puma":        "",
		"gem:rspec-rails": "",
	}
	if len(got) != len(want) {
		t.Errorf("got %d entries (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestGemfileLockDeps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile.lock"), `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.8.7)
      activesupport (= 7.0.8.7)
    activesupport (7.0.8.7)
      concurrent-ruby (~> 1.0, >= 1.0.2)
    concurrent-ruby (1.3.4)
    sidekiq (7.3.0)

PLATFORMS
  ruby

DEPENDENCIES
  rails (~> 7.0)
  sidekiq (>= 7.1)

BUNDLED WITH
   2.5.6
`)
	es := collect(t, GemfileLockDeps{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"gem:rails":           "7.0.8.7",
		"gem:activesupport":   "7.0.8.7",
		"gem:concurrent-ruby": "1.3.4",
		"gem:sidekiq":         "7.3.0",
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
