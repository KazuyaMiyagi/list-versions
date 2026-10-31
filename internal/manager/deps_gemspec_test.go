package manager

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGemspecDeps(t *testing.T) {
	type tv struct{ Type, Version string }
	cases := []struct {
		name string
		body string
		want []tv
	}{
		{"runtime dependency with constraints", `  spec.add_dependency "rails", ">= 7.0", "< 8"`, []tv{{"gem:rails", ">= 7.0, < 8"}}},
		{"runtime and development variants", "spec.add_runtime_dependency \"rack\", \"~> 3.0\"\nspec.add_development_dependency \"rspec\", \"~> 3.13\"", []tv{{"gem:rack", "~> 3.0"}, {"gem:rspec", "~> 3.13"}}},
		{"generated %q form", `  s.add_runtime_dependency(%q<activesupport>.freeze, [">= 6.1".freeze, "< 8".freeze])`, []tv{{"gem:activesupport", ">= 6.1, < 8"}}},
		{"constraints on continuation lines", "spec.add_dependency \"rails\",\n                    \">= 7.0\", \"< 8\"", []tv{{"gem:rails", ">= 7.0, < 8"}}},
		{"comment ending with a comma", "# Runtime dependencies, kept minimal,\nspec.add_dependency \"rack\", \"~> 3.0\"", []tv{{"gem:rack", "~> 3.0"}}},
		{"no constraint", `  spec.add_dependency "zeitwerk"`, []tv{{"gem:zeitwerk", ""}}},
		{"name from a constant skipped", `  spec.add_dependency RAILS_GEM, ">= 7"`, nil},
		{"interpolation skipped", `  spec.add_dependency "rails", "~> #{RAILS_VERSION}"`, nil},
		{"unterminated literal skipped", "spec.add_dependency \"rails\nspec.add_dependency \"rack\"", []tv{{"gem:rack", ""}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "mygem.gemspec"), c.body+"\n")
			var got []tv
			for _, e := range collect(t, GemspecDeps{}, dir) {
				if e.Name != "mygem.gemspec" {
					t.Errorf("got Name %q, want mygem.gemspec", e.Name)
				}
				got = append(got, tv{e.Type, e.Version})
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
