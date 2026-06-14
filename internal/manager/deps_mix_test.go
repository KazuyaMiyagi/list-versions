package manager

import (
	"path/filepath"
	"testing"
)

func TestMixExs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), `
defmodule MyApp.MixProject do
  use Mix.Project

  defp deps do
    [
      {:phoenix, "~> 1.7.10"},
      {:ecto_sql, ">= 3.10.0"},
      {:my_local, path: "../my_local"},
      {:from_git, git: "https://github.com/foo/bar"}
    ]
  end
end
`)
	es := collect(t, MixExs{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"hex:phoenix":  "~> 1.7.10",
		"hex:ecto_sql": ">= 3.10.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["hex:my_local"]; ok {
		t.Errorf("path-based dep should be skipped")
	}
}

func TestMixExs_NonVersionWordsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), `
defp deps do
  [
    {:tagged, "v1.2.3"},
    {:worded, "verbose"},
    {:env_opt, "production"}
  ]
end
`)
	es := collect(t, MixExs{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["hex:tagged"] != "v1.2.3" {
		t.Errorf("v-prefixed version should be kept, got %q", got["hex:tagged"])
	}
	if _, ok := got["hex:worded"]; ok {
		t.Errorf(`"verbose" is not a version and must be skipped`)
	}
	if _, ok := got["hex:env_opt"]; ok {
		t.Errorf(`"production" is not a version and must be skipped`)
	}
}

func TestMixLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.lock"), `%{
  "phoenix": {:hex, :phoenix, "1.7.10", "abc", [:mix], [], "hexpm", "def"},
  "ecto_sql": {:hex, :ecto_sql, "3.11.1", "...", [:mix], [], "hexpm", "..."},
}
`)
	es := collect(t, MixLock{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["hex:phoenix"] != "1.7.10" || got["hex:ecto_sql"] != "3.11.1" {
		t.Errorf("got %v", got)
	}
}
