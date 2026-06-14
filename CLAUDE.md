# list-versions

A "bird's-eye view for upgrade planning" version-declaration inventory CLI. It lists, in a single command, the areas Dependabot doesn't touch (language runtimes / IaC resources / CI tools / managed services / container images).

This file is scoped to the **implementation conventions and procedures** for Claude (or a developer) working in this repo. Related docs:

- User-facing description, usage, and known limitations → `README.md`
- Rationale behind design decisions (the "why") → `docs/design.md`

## Architecture

A Renovate-style **Manager × Entry** structure.

```
internal/
├── entry/        Entry struct (Type / Path / Name / Version)
├── manager/      the various Managers (one Manager per file as a rule)
└── output/       sort / table / csv / json
cmd/list-versions/ main (CLI)
```

### Entry

```go
type Entry struct {
    Type    string  // "Python", "Lambda", "gem:rails", etc.
    Path    string  // the file's directory (the raw walk result, not cwd-relative; only Clean'd)
    Name    string  // the FILE column value (.python-version / Gemfile / Dockerfile / aws_lambda_function.api, etc.)
    Version string  // the VERSION column value
}
```

**Name is a pointer to "where the user should edit"; Version is "what is actually declared."** When one file has multiple FROMs (as in a Dockerfile), Name is fixed to `Dockerfile` and the `image:tag` goes into Version so the differences are still visible (the bash version used Name=image, but we prioritize consistency of the Entry model).

### Manager interface

```go
type Manager interface {
    Name() string
    Match(path string) bool                       // pure path predicate; the walk is shared, so no filesystem access
    Extract(file File) ([]entry.Entry, error)      // pull Entries out of one matched file (use readBytes(file) for the contents)
}

// Optional extension for managers that read a whole directory at once.
type DirScoped interface {
    Manager
    GroupByDir() bool
}
```

There is **one shared filesystem walk per root** (`scanRoot`). Every file is offered to each Manager's `Match`; a file claimed by several Managers is read from disk **once** and the bytes (`File.Data`) are handed to each. `Match` must be a pure predicate (no I/O). Managers that span multiple files (`Terraform`, `TerraformResources`) implement `DirScoped` / `GroupByDir() bool = true`: their matched `*.tf` files are collapsed to their parent directory and `Extract` is called once per dir with `File{Path: dir}` (Data nil). Use the `readBytes(file)` helper inside `Extract` so cached and standalone calls both work.

### Execution flow

`ScanAll(roots, managers, excludes) → [][]Entry` (separated per root) → per root `filepath.Clean(Path)` → `output.SortEntries` → concat in root order → `--type` filter → `EntriesTable/CSV/JSON`.

The prune set is built fresh per `ScanAll` call from read-only defaults + `excludes` (no shared mutable global).

Why preserve root order: the order of arguments passed via shell glob (`list-versions ~/src/*/`) carries meaning in the UI.

## TYPE naming conventions

### Runtime / product names (capitalized)

Use the official spelling:

- `Python` / `Ruby` / `Node.js` / `Go` / `Java` / `Bun` / `Deno` / `Rust` / `PHP` / `Perl` / `Lua` / `Crystal` / `Elixir` / `Erlang` / `Scala` / `Swift`
- `Bundler` / `Yarn` / `OpenAPI Generator` / `Terraform` / `Helm Chart` / `GitHub Actions`
- Follow lowercase when that's official: `npm` / `pnpm` / `sbt` / `pip` (the latter as a deps prefix only)

### Managed services

- AWS: `Lambda` / `Lambda@Edge` / `RDS` / `Aurora` / `ElastiCache (Redis)` / `ElastiCache (Memcached)` / `ElastiCache (Valkey)` / `OpenSearch` / `EKS` / `EKS Node Group` / `ECS` / `App Runner` / `AMI`
- GCP: `Cloud Run` / `Cloud Functions` / `App Engine` / `Cloud SQL` / `Compute Image`
- IaC: `Terraform` / `Terraform Provider` / `Terraform Module`
- Containers: `Docker` / `Kubernetes`

### Package types (under `--deps`, `<registry>:<name>` form, lowercase)

```
gem:rails           rubygems.org
npm:react           npmjs.com  (shared by yarn/pnpm/bun)
pip:django          PyPI
cargo:tokio         crates.io
go:github.com/...   Go module proxy
composer:laravel/framework  Packagist
hex:phoenix         hex.pm
```

The prefix is the **registry name**, not the language name (`gem:rails`, not `Ruby:rails`). Why: when one language has multiple package managers (Node.js → npm/yarn/pnpm/bun), this makes "which registry?" explicit, and it lets us keep a runtime's TYPE separate from its package manager's (as with Bundler).

## Columns and sorting

Column order is **linked** to the order given in `--sort`:

- Default: `directory,file,type,version`
- The table header is likewise `DIRECTORY | FILE | TYPE | VERSION`
- The CSV header and the JSON object key order also reflect the given order

Root (positional argument) order is preserved + sort-key order within each root.

## Template for adding a Manager

To add a new Manager (e.g. `Foo`):

1. Create `internal/manager/foo.go`
   - `type Foo struct{}`
   - `Match(path string) bool` — a pure predicate on `filepath.Base(path)` (or path context, e.g. `underGithubWorkflows`)
   - `Extract(file File) ([]entry.Entry, error)` — get the contents via `data, err := readBytes(file)`
   - For a directory-spanning Manager, also add `GroupByDir() bool { return true }` and `Match` the relevant files (e.g. `*.tf`)
2. Create `internal/manager/foo_test.go` — tests build fixed inputs with `t.TempDir()` + `writeFile`, and obtain results via the `matched(t, Foo{}, dir)` (claimed files) and `collect(t, Foo{}, dir)` (extracted entries, runs the real scanner) helpers
3. Add `manager.Foo{}` to the `managers` slice in `cmd/list-versions/main.go`
4. If it's `--deps`-only, add it to the `depsFlag` branch
5. Update the detection-target table in README

The shared walk applies the prune set automatically (node_modules / .git / .terraform / vendor / .bundle; extendable per run via `--exclude`).

For the "why" behind design choices (why `--deps` is opt-in, why lockfiles are also read, the Lambda LTS-only policy, why there's no config file / cache, flag naming, etc.), see `docs/design.md`.

## Testing policy

Apply the **Red/Green/Refactor TDD** from the user's global CLAUDE.md by default. At least one unit test per Manager.

- Build hermetic fixtures with `t.TempDir()` (no dependency on real files)
- helper: `writeFile(t, path, content)` (defined in `internal/manager/manager_test.go`)
- Write the test for the desired behavior first → confirm it fails → implement → refactor
- Verification against real repositories is supplementary (for regression detection); do not write tests that depend on any specific repository's internal structure

## Development commands

```sh
# during development
gofmt -l .       # list unformatted files (CI fails if `gofmt -l .` produces any output)
gofmt -w .       # apply formatting
go vet ./...
go test ./...
go build -o /tmp/list-versions ./cmd/list-versions

# cross-build (Windows sanity check)
GOOS=windows GOARCH=amd64 go build -o /tmp/list-versions.exe ./cmd/list-versions

# with version embedded
go build -ldflags "-X main.version=vX.Y.Z" -o /tmp/list-versions ./cmd/list-versions
```

## Release procedure

1. `git tag vX.Y.Z`
2. `git push origin vX.Y.Z`
3. `.github/workflows/release.yml` triggers GoReleaser → posts the per-OS/arch binaries + checksums + changelog to the Release
4. Confirm it's installable via `go install github.com/KazuyaMiyagi/list-versions/cmd/list-versions@vX.Y.Z`

For known limitations, see the Limitations section in `README.md`.
