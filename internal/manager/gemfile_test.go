package manager

import (
	"path/filepath"
	"testing"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

func TestGemfile(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"double quotes", `ruby "3.3.0"`, "3.3.0"},
		{"single quotes", `ruby '3.3.0'`, "3.3.0"},
		{"leading whitespace", `   ruby "4.0.5"`, "4.0.5"},
		{"with extra args", `ruby "3.3.0", patchlevel: "0"`, "3.3.0"},
		{"complex form skipped", `ruby file: ".ruby-version"`, ""},
		{"no ruby directive", `source "https://rubygems.org"`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "Gemfile"), c.body+"\n")
			es := collect(t, Gemfile{}, dir)
			if c.want == "" {
				if len(es) != 0 {
					t.Errorf("expected no entries, got %+v", es)
				}
				return
			}
			if len(es) != 1 || es[0].Type != "Ruby" || es[0].Version != c.want {
				t.Errorf("got %+v, want version %q", es, c.want)
			}
		})
	}
}

func TestIsGemfile(t *testing.T) {
	cases := []struct {
		path       string
		gemfile    bool
		gemfileLck bool
	}{
		{"app/Gemfile", true, false},
		{"app/Gemfile.lock", false, true},
		{"app/gems.rb", true, false},
		{"app/gems.locked", false, true},
		{"app/gemfiles/rails_7_1.gemfile", true, false},
		{"app/gemfiles/rails_7_1.gemfile.lock", false, true},
		{"app/Gemfile.local", false, false},
		{"app/gemfile.rb", false, false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := isGemfile(c.path); got != c.gemfile {
				t.Errorf("isGemfile: got %v, want %v", got, c.gemfile)
			}
			if got := isGemfileLock(c.path); got != c.gemfileLck {
				t.Errorf("isGemfileLock: got %v, want %v", got, c.gemfileLck)
			}
		})
	}
}

func TestGemfileManagers_GemsRb(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gems.rb"), "ruby \"3.4.1\"\ngem \"rails\", \"~> 8.0\"\n")
	writeFile(t, filepath.Join(dir, "gems.locked"), "GEM\n  specs:\n    rails (8.0.2)\n\nBUNDLED WITH\n   2.6.9\n")

	cases := []struct {
		m    Manager
		want entry.Entry
	}{
		{Gemfile{}, entry.Entry{Type: "Ruby", Name: "gems.rb", Version: "3.4.1"}},
		{GemfileDeps{}, entry.Entry{Type: "gem:rails", Name: "gems.rb", Version: "~> 8.0"}},
		{GemfileLock{}, entry.Entry{Type: "Bundler", Name: "gems.locked", Version: "2.6.9"}},
		{GemfileLockDeps{}, entry.Entry{Type: "gem:rails", Name: "gems.locked", Version: "8.0.2"}},
	}
	for _, c := range cases {
		t.Run(c.m.Name(), func(t *testing.T) {
			es := collect(t, c.m, dir)
			if len(es) != 1 {
				t.Fatalf("got %+v, want one entry", es)
			}
			got := es[0]
			got.Path = ""
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
