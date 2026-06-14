package manager

import (
	"path/filepath"
	"testing"
)

func TestComposerJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.json"), `{
  "require": {
    "php": "^8.2",
    "laravel/framework": "^11.0"
  },
  "require-dev": {
    "phpunit/phpunit": "^10.0"
  }
}`)
	es := collect(t, ComposerJSON{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"composer:php":               "^8.2",
		"composer:laravel/framework": "^11.0",
		"composer:phpunit/phpunit":   "^10.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["composer:laravel/framework"] != "composer.json" {
		t.Errorf("prod dep Name: got %q", gotName["composer:laravel/framework"])
	}
	if gotName["composer:phpunit/phpunit"] != "composer.json (dev)" {
		t.Errorf("dev dep Name: got %q", gotName["composer:phpunit/phpunit"])
	}
}

func TestComposerLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "composer.lock"), `{
  "packages": [
    {"name": "laravel/framework", "version": "v11.5.0"},
    {"name": "symfony/console", "version": "v7.0.4"}
  ],
  "packages-dev": [
    {"name": "phpunit/phpunit", "version": "10.5.10"}
  ]
}`)
	es := collect(t, ComposerLock{}, dir)
	got := map[string]string{}
	gotName := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
		gotName[e.Type] = e.Name
	}
	want := map[string]string{
		"composer:laravel/framework": "v11.5.0",
		"composer:symfony/console":   "v7.0.4",
		"composer:phpunit/phpunit":   "10.5.10",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if gotName["composer:laravel/framework"] != "composer.lock" {
		t.Errorf("prod dep Name: got %q", gotName["composer:laravel/framework"])
	}
	if gotName["composer:phpunit/phpunit"] != "composer.lock (dev)" {
		t.Errorf("dev dep Name: got %q", gotName["composer:phpunit/phpunit"])
	}
}
