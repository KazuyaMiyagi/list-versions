package output

import (
	"reflect"
	"testing"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

func TestNatCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.9.0", "3.10.0", -1},
		{"3.10.0", "3.9.0", 1},
		{"v7.10.0", "v7.11.0", -1},
		{"1.0", "1.0", 0},
		{"ruby-4.0.5", "ruby-3.4.9", 1},
		{"", "1", -1},
	}
	for _, c := range cases {
		got := natCompare(c.a, c.b)
		if (got < 0 && c.want >= 0) || (got > 0 && c.want <= 0) || (got == 0 && c.want != 0) {
			t.Errorf("natCompare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseKeys(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{"directory", "file", "type", "version"}},
		{"directory,file,type,version", []string{"directory", "file", "type", "version"}},
		{"type", []string{"type", "directory", "file", "version"}},
		{"version,file", []string{"version", "file", "directory", "type"}},
	}
	for _, c := range cases {
		got, err := ParseKeys(c.in)
		if err != nil {
			t.Errorf("ParseKeys(%q) error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseKeys(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseKeys_Errors(t *testing.T) {
	for _, in := range []string{"foo", "type,foo", "type,type"} {
		if _, err := ParseKeys(in); err == nil {
			t.Errorf("ParseKeys(%q) expected error", in)
		}
	}
}

func TestSortEntries_KeyOrder(t *testing.T) {
	es := []entry.Entry{
		{Path: "b", Type: "Ruby", Version: "4.0.5", Name: ".ruby-version"},
		{Path: "a", Type: "Node.js", Version: "24.0.0", Name: ".node-version"},
		{Path: "a", Type: "Ruby", Version: "3.0.0", Name: "Gemfile"},
	}
	SortEntries(es, []string{"type", "directory", "version", "file"})
	// Type-first ordering: Node.js, then Ruby (a then b).
	wantPaths := []string{"a", "a", "b"}
	gotPaths := []string{es[0].Path, es[1].Path, es[2].Path}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("got paths %v, want %v", gotPaths, wantPaths)
	}
	if es[0].Type != "Node.js" || es[1].Type != "Ruby" {
		t.Errorf("type ordering broken: %+v", es)
	}
}

// signature collapses an Entry to a short string so test expectations stay
// readable.
func signature(e entry.Entry) string {
	return e.Path + "/" + e.Type + "/" + e.Version + "/" + e.Name
}

func signatures(es []entry.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = signature(e)
	}
	return out
}

// TestSortEntries_TypeOnly mirrors `--sort type`: ParseKeys expands it to
// (type, directory, file, version) and SortEntries should honour that order.
func TestSortEntries_TypeOnly(t *testing.T) {
	es := []entry.Entry{
		{Path: "z", Type: "Ruby", Version: "3.0", Name: "Gemfile"},
		{Path: "a", Type: "Node.js", Version: "20", Name: ".node-version"},
		{Path: "m", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
		{Path: "a", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
	}
	keys, err := ParseKeys("type")
	if err != nil {
		t.Fatal(err)
	}
	SortEntries(es, keys)
	want := []string{
		"a/Node.js/20/.node-version",
		"a/Ruby/4.0/.ruby-version",
		"m/Ruby/4.0/.ruby-version",
		"z/Ruby/3.0/Gemfile",
	}
	if got := signatures(es); !reflect.DeepEqual(got, want) {
		t.Errorf("\nwant %v\n got %v", want, got)
	}
}

// TestSortEntries_FullExplicit pins the behaviour when the user spells out
// every key in a non-default order (`--sort file,version,type,directory`).
func TestSortEntries_FullExplicit(t *testing.T) {
	es := []entry.Entry{
		{Path: "b", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
		{Path: "a", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
		{Path: "a", Type: "Node.js", Version: "20", Name: ".node-version"},
		{Path: "a", Type: "Ruby", Version: "3.0", Name: ".ruby-version"},
	}
	keys, err := ParseKeys("file,version,type,directory")
	if err != nil {
		t.Fatal(err)
	}
	SortEntries(es, keys)
	want := []string{
		// .node-version comes first by name.
		"a/Node.js/20/.node-version",
		// Then .ruby-version, ordered by version ascending, then path.
		"a/Ruby/3.0/.ruby-version",
		"a/Ruby/4.0/.ruby-version",
		"b/Ruby/4.0/.ruby-version",
	}
	if got := signatures(es); !reflect.DeepEqual(got, want) {
		t.Errorf("\nwant %v\n got %v", want, got)
	}
}

// TestSortEntries_DefaultKeys covers the default `directory,file,type,version`
// ordering returned by ParseKeys when no flag is supplied.
func TestSortEntries_DefaultKeys(t *testing.T) {
	es := []entry.Entry{
		{Path: "b", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
		{Path: "a", Type: "Ruby", Version: "4.0", Name: "Gemfile"},
		{Path: "a", Type: "Node.js", Version: "20", Name: ".node-version"},
		{Path: "a", Type: "Ruby", Version: "4.0", Name: ".ruby-version"},
	}
	keys, err := ParseKeys("")
	if err != nil {
		t.Fatal(err)
	}
	SortEntries(es, keys)
	want := []string{
		"a/Node.js/20/.node-version",
		"a/Ruby/4.0/.ruby-version",
		"a/Ruby/4.0/Gemfile",
		"b/Ruby/4.0/.ruby-version",
	}
	if got := signatures(es); !reflect.DeepEqual(got, want) {
		t.Errorf("\nwant %v\n got %v", want, got)
	}
}
