package manager

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGemspec(t *testing.T) {
	type tv struct{ Type, Version string }
	cases := []struct {
		name string
		body string
		want []tv
	}{
		{"ruby requirement", `  spec.required_ruby_version = ">= 3.1.0"`, []tv{{"Ruby", ">= 3.1.0"}}},
		{"single quotes", `  spec.required_ruby_version = '>= 2.7'`, []tv{{"Ruby", ">= 2.7"}}},
		{"array of requirements", `  s.required_ruby_version = [">= 3.0", "< 4"]`, []tv{{"Ruby", ">= 3.0, < 4"}}},
		{"Gem::Requirement", `  s.required_ruby_version = Gem::Requirement.new(">= 2.7.0".freeze)`, []tv{{"Ruby", ">= 2.7.0"}}},
		{"rubygems requirement", `  spec.required_rubygems_version = ">= 3.3.0"`, []tv{{"RubyGems", ">= 3.3.0"}}},
		{"ruby and rubygems", "spec.required_ruby_version = \">= 3.1\"\nspec.required_rubygems_version = \">= 3.3.0\"", []tv{{"Ruby", ">= 3.1"}, {"RubyGems", ">= 3.3.0"}}},
		{"rubygems >= 0 skipped", `  s.required_rubygems_version = Gem::Requirement.new(">= 0".freeze) if s.respond_to? :required_rubygems_version=`, nil},
		{"constant reference skipped", `  spec.required_ruby_version = MyGem::RUBY_REQUIREMENT`, nil},
		{"interpolation skipped", `  spec.required_ruby_version = ">= #{MIN_RUBY}"`, nil},
		{"commented out skipped", `  # spec.required_ruby_version = ">= 3.1"`, nil},
		{"no requirement", `  spec.name = "mygem"`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "mygem.gemspec"), c.body+"\n")
			var got []tv
			for _, e := range collect(t, Gemspec{}, dir) {
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

func TestGemspec_MultipleFilesInDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.gemspec"), "spec.required_ruby_version = \">= 3.1\"\n")
	writeFile(t, filepath.Join(dir, "b.gemspec"), "spec.required_ruby_version = \">= 3.2\"\n")

	got := map[string]string{}
	for _, e := range collect(t, Gemspec{}, dir) {
		got[e.Name] = e.Version
	}
	want := map[string]string{"a.gemspec": ">= 3.1", "b.gemspec": ">= 3.2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
