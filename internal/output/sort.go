package output

import (
	"sort"
	"unicode"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// natCompare compares two strings as a sequence of alternating non-digit and
// digit runs. Numeric runs are compared numerically so "3.10.0" sorts after
// "3.9.0".
func natCompare(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		ad := unicode.IsDigit(ar[i])
		bd := unicode.IsDigit(br[j])
		if ad && bd {
			ai := i
			for ai < len(ar) && unicode.IsDigit(ar[ai]) {
				ai++
			}
			bj := j
			for bj < len(br) && unicode.IsDigit(br[bj]) {
				bj++
			}
			an := trimZeroes(ar[i:ai])
			bn := trimZeroes(br[j:bj])
			if len(an) != len(bn) {
				if len(an) < len(bn) {
					return -1
				}
				return 1
			}
			for k := 0; k < len(an); k++ {
				if an[k] != bn[k] {
					if an[k] < bn[k] {
						return -1
					}
					return 1
				}
			}
			i, j = ai, bj
			continue
		}
		if ar[i] != br[j] {
			if ar[i] < br[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case i < len(ar):
		return 1
	case j < len(br):
		return -1
	}
	return 0
}

func trimZeroes(rs []rune) []rune {
	i := 0
	for i < len(rs)-1 && rs[i] == '0' {
		i++
	}
	return rs[i:]
}

// SortEntries orders entries by the supplied keys, using natural-version
// comparison on each field. Callers typically sort within a single root so
// that the glob-argument order is preserved across roots in the final
// flattened output.
func SortEntries(es []entry.Entry, keys []string) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		for _, k := range keys {
			if c := natCompare(fieldByKey(a, k), fieldByKey(b, k)); c != 0 {
				return c < 0
			}
		}
		return false
	})
}

func fieldByKey(e entry.Entry, key string) string {
	switch key {
	case "directory":
		return e.Path
	case "type":
		return e.Type
	case "version":
		return e.Version
	case "file":
		return e.Name
	}
	return ""
}
