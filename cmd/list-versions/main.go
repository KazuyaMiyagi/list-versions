package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
	"github.com/KazuyaMiyagi/list-versions/internal/manager"
	"github.com/KazuyaMiyagi/list-versions/internal/output"
)

// version is overridden at build time with `-ldflags "-X main.version=vX.Y.Z"`.
// `dev` is the default for local `go build` invocations.
var version = "dev"

const usage = `Usage: list-versions [OPTIONS] [PATH...]

List language-runtime and tool version declarations under each PATH.
If no PATH is given, the current directory is used. Multiple paths are
supported (typically via shell glob, e.g. list-versions ~/src/*/).

Detected sources:
  .python-version, .ruby-version, .node-version, .terraform-version,
  .go-version, .nvmrc, .tool-versions, package.json (engines.node),
  Gemfile (ruby), pyproject.toml (requires-python / Poetry python),
  Dockerfile (FROM image:tag), Terraform (required_version,
  required_providers, module_calls, plus AWS resource runtime/engine
  versions on Lambda / RDS / Aurora / ElastiCache / OpenSearch / EKS),
  GitHub Actions workflows (uses: foo/bar@vN, with: *-version),
  Kubernetes manifests (container images on Pod / Deployment /
  StatefulSet / DaemonSet / ReplicaSet / Job / CronJob).

Options:
  -f, --format FORMAT   output format: table (default), csv, json
  -s, --sort KEYS       comma-separated sort keys from {directory, type,
                        version, file}; the same order is used for the
                        output columns. Default: directory,file,type,version
  -t, --type TYPES      keep only rows whose TYPE matches one of TYPES
                        (comma-separated, case-insensitive). Each TYPE may
                        contain glob wildcards (* and ?), e.g.
                        --type 'gem:rails*,npm:@types/*'
  -x, --exclude DIRS    extra directory basenames to skip during the walk
                        (comma-separated, e.g. --exclude .venv,tmp)
  -d, --deps            include application-level direct dependencies from
                        Gemfile / Gemfile.lock / package.json /
                        package-lock.json / yarn.lock / pnpm-lock.yaml /
                        pyproject.toml / poetry.lock / requirements.txt /
                        Cargo.toml / Cargo.lock / go.mod
  -v, --version         print the binary version and exit
  -h, --help            show this message
`

func main() {
	fs := flag.NewFlagSet("list-versions", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	// Suppress flag's automatic usage dump; the Parse error is handled below so
	// help goes to stdout (exit 0) and genuine usage errors go to stderr.
	fs.Usage = func() {}

	var format string
	var sortFlag string
	var typeFlag string
	var depsFlag bool
	fs.StringVar(&format, "format", "table", "")
	fs.StringVar(&format, "f", "table", "")
	fs.StringVar(&sortFlag, "sort", "directory,file,type,version", "")
	fs.StringVar(&sortFlag, "s", "directory,file,type,version", "")
	fs.StringVar(&typeFlag, "type", "", "")
	fs.StringVar(&typeFlag, "t", "", "")
	fs.BoolVar(&depsFlag, "deps", false, "")
	fs.BoolVar(&depsFlag, "d", false, "")
	var excludeFlag string
	fs.StringVar(&excludeFlag, "exclude", "", "")
	fs.StringVar(&excludeFlag, "x", "", "")
	var versionFlag bool
	fs.BoolVar(&versionFlag, "version", false, "")
	fs.BoolVar(&versionFlag, "v", false, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		// -h/--help is an explicit request, not a usage error: print the help
		// text to stdout and exit cleanly.
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(os.Stdout, usage)
			return
		}
		// Genuine usage error: flag already wrote the message to stderr; add
		// the usage text and fail.
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if versionFlag {
		fmt.Println("list-versions", version)
		return
	}

	var excludes []string
	for _, x := range strings.Split(excludeFlag, ",") {
		x = strings.TrimSpace(x)
		if x != "" {
			excludes = append(excludes, x)
		}
	}

	keys, err := output.ParseKeys(sortFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list-versions:", err)
		os.Exit(2)
	}

	roots := fs.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	managers := []manager.Manager{
		manager.VersionFile{},
		manager.Nvmrc{},
		manager.ToolVersions{},
		manager.PackageJSON{},
		manager.Gemfile{},
		manager.GemfileLock{},
		manager.PyprojectToml{},
		manager.Dockerfile{},
		manager.Terraform{},
		manager.TerraformResources{},
		manager.GithubActions{},
		manager.Kubernetes{},
		manager.CloudFormation{},
		manager.Helm{},
		manager.Serverless{},
	}
	if depsFlag {
		managers = append(managers,
			manager.GemfileDeps{},
			manager.GemfileLockDeps{},
			manager.PackageJSONDeps{},
			manager.PackageLockJSON{},
			manager.YarnLock{},
			manager.PnpmLock{},
			manager.BunLock{},
			manager.PyprojectTomlDeps{},
			manager.PoetryLock{},
			manager.RequirementsTxt{},
			manager.Pipfile{},
			manager.PipfileLock{},
			manager.UvLock{},
			manager.ComposerJSON{},
			manager.ComposerLock{},
			manager.MixExs{},
			manager.MixLock{},
			manager.CargoToml{},
			manager.CargoLock{},
			manager.GoMod{},
		)
	}

	// scanErr is non-nil when one or more roots were inaccessible. ScanAll has
	// already warned about each on stderr, so we still emit whatever the
	// accessible roots produced and exit non-zero at the end.
	groups, scanErr := manager.ScanAll(roots, managers, excludes, os.Stderr)

	typeMatch := parseTypeFilter(typeFlag)

	var entries []entry.Entry
	totalBeforeFilter := 0
	for _, g := range groups {
		for i := range g {
			g[i].Path = filepath.Clean(g[i].Path)
		}
		output.SortEntries(g, keys)
		totalBeforeFilter += len(g)
		for _, e := range g {
			if typeMatch != nil && !typeMatch(e.Type) {
				continue
			}
			entries = append(entries, e)
		}
	}

	if len(entries) == 0 {
		// Suppress the friendly "nothing found" line when a root failed: the
		// per-root warning already explains the empty result.
		if scanErr == nil {
			fmt.Fprintln(os.Stderr, emptyOutputMessage(typeMatch != nil, totalBeforeFilter))
		}
	} else if err := emit(format, entries, keys); err != nil {
		fmt.Fprintln(os.Stderr, "list-versions:", err)
		os.Exit(1)
	}

	if scanErr != nil {
		os.Exit(1)
	}
}

// parseTypeFilter returns nil when no filter is active so the caller can
// short-circuit. Otherwise it returns a predicate that matches a TYPE value
// against the comma-separated list, case-insensitively. Entries that contain
// `*` or `?` are treated as globs (with `*` matching any number of characters
// including separators, `?` matching exactly one); other entries match
// exactly.
func parseTypeFilter(s string) func(string) bool {
	if s == "" {
		return nil
	}
	var matchers []func(string) bool
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if strings.ContainsAny(lower, "*?") {
			re := globToRegex(lower)
			matchers = append(matchers, re.MatchString)
		} else {
			literal := lower
			matchers = append(matchers, func(s string) bool { return s == literal })
		}
	}
	if len(matchers) == 0 {
		return nil
	}
	return func(typ string) bool {
		lower := strings.ToLower(typ)
		for _, m := range matchers {
			if m(lower) {
				return true
			}
		}
		return false
	}
}

func globToRegex(pat string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pat {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '|', '^', '$', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// emptyOutputMessage picks the stderr message shown when the table would be
// empty. A non-nil filter that hid every scanned entry gets its own line so
// users don't think the scan itself failed to find anything.
func emptyOutputMessage(filterActive bool, scannedBeforeFilter int) string {
	if filterActive && scannedBeforeFilter > 0 {
		return "No entries matched --type filter."
	}
	return "No version files found."
}

func emit(format string, es []entry.Entry, keys []string) error {
	switch format {
	case "table":
		return output.EntriesTable(os.Stdout, es, keys)
	case "csv":
		return output.EntriesCSV(os.Stdout, es, keys)
	case "json":
		return output.EntriesJSON(os.Stdout, es, keys)
	}
	return fmt.Errorf("unknown format: %s", format)
}
