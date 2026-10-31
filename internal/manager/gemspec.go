package manager

import (
	"bufio"
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// The right-hand side is not evaluated as Ruby: every string literal in the
// statement is taken as one requirement, which covers the plain string, array and
// Gem::Requirement.new forms. Constant references and `#{}` interpolation
// would need evaluation, so they are skipped rather than shown half-resolved.
var (
	gemspecRequirementRe = regexp.MustCompile(`^\s*\w+\.(required_ruby_version|required_rubygems_version)\s*=(.*)$`)
	gemspecStringLitRe   = regexp.MustCompile(`"([^"]*)"|'([^']*)'|%q<([^>]*)>`)
)

var gemspecRequirementType = map[string]string{
	"required_ruby_version":     "Ruby",
	"required_rubygems_version": "RubyGems",
}

// Gemspec reads `required_ruby_version` and `required_rubygems_version` from
// a *.gemspec file.
type Gemspec struct{}

func (Gemspec) Name() string { return "gemspec" }

func (Gemspec) Match(path string) bool { return filepath.Ext(path) == ".gemspec" }

func (Gemspec) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var entries []entry.Entry
	seen := map[string]bool{}
	stmts, err := rubyStatements(data)
	if err != nil {
		return nil, err
	}
	for _, stmt := range stmts {
		m := gemspecRequirementRe.FindStringSubmatch(stmt)
		if m == nil || seen[m[1]] {
			continue
		}
		reqs := rubyStringLiterals(m[2])
		if len(reqs) == 0 || strings.Contains(m[2], "#{") {
			continue
		}
		seen[m[1]] = true
		version := strings.Join(reqs, ", ")
		// `>= 0` is the value gem generators write by default and is the same
		// as no constraint, so listing it would only add noise.
		if m[1] == "required_rubygems_version" && version == ">= 0" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    gemspecRequirementType[m[1]],
			Path:    filepath.Dir(file.Path),
			Name:    filepath.Base(file.Path),
			Version: version,
		})
	}
	return entries, nil
}

func rubyStringLiterals(s string) []string {
	var lits []string
	for _, m := range gemspecStringLitRe.FindAllStringSubmatch(s, -1) {
		lits = append(lits, m[1]+m[2]+m[3])
	}
	return lits
}

// rubyStatements splits Ruby source into lines, joining a line that ends with
// `,`, `(` or `[` with the next one, so an argument list or array that spans
// several lines is read as a single statement.
func rubyStatements(data []byte) ([]string, error) {
	var stmts []string
	var cur strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(line)
		if continuesOnNextLine(line) {
			continue
		}
		stmts = append(stmts, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		stmts = append(stmts, cur.String())
	}
	return stmts, scanner.Err()
}

func continuesOnNextLine(line string) bool {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasSuffix(t, ",") || strings.HasSuffix(t, "(") || strings.HasSuffix(t, "[")
}
