package main

import "testing"

func TestEmptyOutputMessage(t *testing.T) {
	cases := []struct {
		name                string
		filterActive        bool
		scannedBeforeFilter int
		want                string
	}{
		{
			name:                "no filter, nothing scanned",
			filterActive:        false,
			scannedBeforeFilter: 0,
			want:                "No version files found.",
		},
		{
			name:                "no filter, entries existed but caller still empty",
			filterActive:        false,
			scannedBeforeFilter: 5,
			want:                "No version files found.",
		},
		{
			name:                "filter active, scanned 0 (genuinely empty repo)",
			filterActive:        true,
			scannedBeforeFilter: 0,
			want:                "No version files found.",
		},
		{
			name:                "filter active, scanned >0 (filter hid everything)",
			filterActive:        true,
			scannedBeforeFilter: 12,
			want:                "No entries matched --type filter.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := emptyOutputMessage(c.filterActive, c.scannedBeforeFilter)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseTypeFilter(t *testing.T) {
	cases := []struct {
		name      string
		filter    string
		typ       string
		wantMatch bool
	}{
		{"empty filter returns nil", "", "anything", true}, // nil → no filter applied
		{"exact match (case-insensitive)", "ruby", "Ruby", true},
		{"exact non-match", "ruby", "Python", false},
		{"glob prefix", "gem:rails*", "gem:rails-controller-testing", true},
		{"glob prefix no match", "gem:rails*", "gem:sidekiq", false},
		{"glob single-char", "Lambda?Edge", "Lambda@Edge", true},
		{"multiple comma", "Ruby,Python", "Python", true},
		{"multiple comma no match", "Ruby,Python", "Go", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match := parseTypeFilter(c.filter)
			var got bool
			if match == nil {
				got = true
			} else {
				got = match(c.typ)
			}
			if got != c.wantMatch {
				t.Errorf("filter=%q typ=%q got=%v want=%v", c.filter, c.typ, got, c.wantMatch)
			}
		})
	}
}
