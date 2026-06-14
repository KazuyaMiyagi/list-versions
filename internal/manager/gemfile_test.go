package manager

import (
	"path/filepath"
	"testing"
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
