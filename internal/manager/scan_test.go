package manager

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// okManager claims a single basename and returns canned entries for it.
type okManager struct {
	base string
	es   []entry.Entry
}

func (okManager) Name() string                          { return "ok" }
func (m okManager) Match(path string) bool              { return filepath.Base(path) == m.base }
func (m okManager) Extract(File) ([]entry.Entry, error) { return m.es, nil }

// errExtractManager claims a basename but fails in Extract, as a manager would
// on a malformed file it nonetheless decided to parse.
type errExtractManager struct {
	base string
}

func (errExtractManager) Name() string             { return "err-extract" }
func (m errExtractManager) Match(path string) bool { return filepath.Base(path) == m.base }
func (errExtractManager) Extract(File) ([]entry.Entry, error) {
	return nil, errors.New("simulated parse error")
}

func TestScanAll_ContinuesOnExtractError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good"), "x")
	writeFile(t, filepath.Join(dir, "bad"), "x")
	ok := okManager{
		base: "good",
		es:   []entry.Entry{{Type: "Python", Path: dir, Name: ".python-version", Version: "3.11.0"}},
	}
	bad := errExtractManager{base: "bad"}

	var warn bytes.Buffer
	groups, err := ScanAll([]string{dir}, []Manager{ok, bad}, nil, &warn)
	if err != nil {
		t.Fatalf("ScanAll returned fatal error: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 1 {
		t.Fatalf("expected 1 group of 1 entry from the OK manager, got %+v", groups)
	}
	if groups[0][0].Version != "3.11.0" {
		t.Errorf("OK entry lost: %+v", groups[0][0])
	}
	if !strings.Contains(warn.String(), "bad") {
		t.Errorf("warning should mention the failing file, got %q", warn.String())
	}
	if !strings.Contains(warn.String(), "err-extract") {
		t.Errorf("warning should mention the manager Name(), got %q", warn.String())
	}
}

// panicManager triggers a runtime panic during Extract. ScanAll must catch it
// the same way it catches plain errors, so the rest of the scan continues and
// the user gets a warning instead of an empty report + crashed process.
type panicManager struct {
	base string
}

func (panicManager) Name() string             { return "panicker" }
func (m panicManager) Match(path string) bool { return filepath.Base(path) == m.base }
func (panicManager) Extract(f File) ([]entry.Entry, error) {
	panic("manager exploded while reading " + f.Path)
}

func TestScanAll_RecoversFromPanic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good"), "x")
	writeFile(t, filepath.Join(dir, "explode"), "x")
	ok := okManager{
		base: "good",
		es:   []entry.Entry{{Type: "Python", Path: dir, Name: ".python-version", Version: "3.11.0"}},
	}
	boom := panicManager{base: "explode"}

	var warn bytes.Buffer
	groups, err := ScanAll([]string{dir}, []Manager{ok, boom}, nil, &warn)
	if err != nil {
		t.Fatalf("ScanAll returned fatal error after a panicking manager: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 1 || groups[0][0].Version != "3.11.0" {
		t.Fatalf("OK entries should survive a sibling manager's panic, got %+v", groups)
	}
	if !strings.Contains(warn.String(), "explode") {
		t.Errorf("warning should mention the file that triggered the panic, got %q", warn.String())
	}
	if !strings.Contains(warn.String(), "panicker") {
		t.Errorf("warning should mention the offending manager Name(), got %q", warn.String())
	}
}

func TestScanAll_WarnsAndFailsOnMissingRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good"), "x")
	ok := okManager{
		base: "good",
		es:   []entry.Entry{{Type: "Python", Path: dir, Name: ".python-version", Version: "3.11.0"}},
	}
	missing := filepath.Join(dir, "does-not-exist")

	var warn bytes.Buffer
	groups, err := ScanAll([]string{dir, missing}, []Manager{ok}, nil, &warn)
	if err == nil {
		t.Fatal("expected a non-nil error signalling the inaccessible root")
	}
	// The accessible root must still yield its entries (warn-and-continue).
	if len(groups) != 2 || len(groups[0]) != 1 || groups[0][0].Version != "3.11.0" {
		t.Fatalf("accessible root should still yield entries, got %+v", groups)
	}
	if !strings.Contains(warn.String(), "does-not-exist") {
		t.Errorf("warning should mention the inaccessible root, got %q", warn.String())
	}
}

func TestScanAll_NilWarnWriter(t *testing.T) {
	// Passing nil should be tolerated (silent on Extract errors).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x"), "x")
	bad := errExtractManager{base: "x"}
	if _, err := ScanAll([]string{dir}, []Manager{bad}, nil, nil); err != nil {
		t.Fatalf("nil warnW should not surface as a fatal error: %v", err)
	}
}
