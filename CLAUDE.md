# list-versions

A version-declaration inventory CLI that gives a bird's-eye view
for upgrade planning.
In a single command, it lists the areas Dependabot doesn't touch.
Those are language runtimes, IaC resources, CI tools, cloud services
and container images.

This file covers the implementation conventions and procedures.
It is written for Claude, or a developer, working in this repo.
Related docs:

- User-facing description, usage, and known limitations → `README.md`
- Rationale behind design decisions (the "why") → `docs/design.md`

## Architecture

The code follows a Renovate-style structure of Managers that produce Entries.

```text
internal/
├── entry/        Entry struct (Type, Path, Name, Version)
├── manager/      the various Managers (one Manager per file as a rule)
└── output/       sort, table, csv, json
cmd/list-versions/ main (CLI)
```

### Entry

```go
type Entry struct {
    // "Python", "Lambda", "gem:rails", etc.
    Type string
    // the file's directory: the raw walk result, only Clean'd
    // (not made cwd-relative)
    Path string
    // the FILE column value, e.g. .python-version, Gemfile,
    // Dockerfile or aws_lambda_function.api
    Name string
    // the VERSION column value
    Version string
}
```

Name points at where the user should edit, and Version is what is actually declared.
A Dockerfile can have several FROM lines.
Name then stays `Dockerfile`, and each `image:tag` goes into Version.
That keeps the differences visible.
The bash version put the image in Name instead.
Here the consistency of the Entry model takes priority.

### Manager interface

```go
type Manager interface {
    Name() string
    // pure path predicate; the walk is shared, so no filesystem access
    Match(path string) bool
    // pull Entries out of one matched file
    // (use readBytes(file) for the contents)
    Extract(file File) ([]entry.Entry, error)
}

// Optional extension for managers that read a whole directory at once.
type DirScoped interface {
    Manager
    GroupByDir() bool
}
```

There is one shared filesystem walk per root (`scanRoot`).
Every file is offered to the `Match` of each Manager.
A file that several Managers claim is read from disk only once.
Its bytes (`File.Data`) are then handed to each of them.
That is why `Match` must be a pure predicate with no I/O.

Some Managers span multiple files, namely `Terraform` and `TerraformResources`.
They implement `DirScoped` and return true from `GroupByDir`.
Their matched `*.tf` files are collapsed into the parent directory.
`Extract` is then called once per directory with `File{Path: dir}` and nil Data.
Read file contents through the `readBytes(file)` helper inside `Extract`.
It works both with the cached bytes and in a standalone call.

### Execution flow

1. `ScanAll(roots, managers, excludes)` returns `[][]Entry`, one slice per root
2. Each root's entries go through `filepath.Clean(Path)` and `output.SortEntries`
3. The roots are concatenated in argument order
4. The `--type` filter is applied
5. `EntriesTable`, `EntriesCSV` or `EntriesJSON` writes the output

Each `ScanAll` call builds a fresh prune set from the read-only defaults and `excludes`.
There is no shared mutable global.

Root order is preserved because it carries meaning in the UI.
The roots usually come from a shell glob like `list-versions ~/src/*/`.

## TYPE naming conventions

### Runtime and product names (capitalized)

Use the official spelling:

- Languages: `Python`/`Ruby`/`Node.js`/`Go`/`Java`/`Bun`/`Deno`/`Rust`/`PHP`
- More languages: `Perl`/`Lua`/`Crystal`/`Elixir`/`Erlang`/`Scala`/`Swift`
- Tools: `Bundler`/`RubyGems`/`Yarn`/`Poetry`/`Pipenv`/`OpenAPI Generator`
- More tools: `Terraform`/`Helm Chart`/`GitHub Actions`
- Lowercase where that is the official spelling: `npm`/`pnpm`/`sbt`/`uv`/`pip`
  - `pip` appears only as a deps prefix

### Cloud services

- AWS runtimes and databases: `Lambda`/`Lambda@Edge`/`RDS`/`Aurora`
- AWS caches: `ElastiCache (Redis)`/`ElastiCache (Memcached)`/`ElastiCache (Valkey)`
- Other AWS: `OpenSearch`/`EKS`/`EKS Node Group`/`ECS`/`App Runner`/`AMI`
- GCP: `Cloud Run`/`Cloud Functions`/`App Engine`/`Cloud SQL`/`Compute Image`
- IaC: `Terraform`/`Terraform Provider`/`Terraform Module`
- Containers: `Docker`/`Kubernetes`

### Package types (under `--deps`, `<registry>:<name>` form, lowercase)

```text
gem:rails           rubygems.org
npm:react           npmjs.com  (shared by yarn/pnpm/bun)
pip:django          PyPI
cargo:tokio         crates.io
go:github.com/...   Go module proxy
composer:laravel/framework  Packagist
hex:phoenix         hex.pm
```

The prefix is the registry name, not the language name.
So it is `gem:rails`, not `Ruby:rails`.
One language can have several package managers, as Node.js has npm and Yarn.
The registry prefix makes it explicit where a package comes from.
It also keeps a runtime's TYPE apart from its package manager's TYPE.
Ruby and Bundler are an example of that split.

## Columns and sorting

Column order is linked to the order given in `--sort`:

- Default: `directory,file,type,version`
- The table header is likewise `DIRECTORY | FILE | TYPE | VERSION`
- The CSV header and the JSON object key order also reflect the given order

The order of the roots (positional arguments) is preserved.
The sort keys apply within each root.

## Template for adding a Manager

To add a new Manager (e.g. `Foo`):

1. Create `internal/manager/foo.go`
   - `type Foo struct{}`
   - `Match(path string) bool` is a pure predicate on `filepath.Base(path)`.
     It can also use path context, as `underGithubWorkflows` does.
   - `Extract(file File) ([]entry.Entry, error)` reads the contents
     via `data, err := readBytes(file)`.
   - A directory-spanning Manager also adds `GroupByDir() bool { return true }`.
     Its `Match` then picks the relevant files, such as `*.tf`.
2. Create `internal/manager/foo_test.go`
   - Tests build fixed inputs with `t.TempDir()` and `writeFile`.
   - `matched(t, Foo{}, dir)` returns the files the Manager claims.
   - `collect(t, Foo{}, dir)` runs the real scanner and returns the entries.
3. Add `manager.Foo{}` to the `managers` slice in `cmd/list-versions/main.go`
4. If it's `--deps`-only, add it to the `depsFlag` branch
5. Update the detection-target table in README

The shared walk applies the prune set automatically.
The defaults skip node_modules, .git, .terraform, vendor and .bundle.
`--exclude` adds more directories for a single run.

For the "why" behind design choices, see `docs/design.md`.
It explains why `--deps` is opt-in and why lockfiles are also read.
It also covers the Lambda LTS-only policy and the flag naming.
The reasons for having no configuration file or cache are there too.

## Testing policy

Apply the Red/Green/Refactor TDD from the user's global CLAUDE.md by default.
Write at least one unit test for every Manager.

- Build hermetic fixtures with `t.TempDir()`, with no dependency on real files
- Use the `writeFile(t, path, content)` helper from `internal/manager/manager_test.go`
- Write the test for the desired behavior first and confirm that it fails.
  Then implement, and refactor afterwards.
- Checking against real repositories is a supplement for catching regressions.
  Do not write tests that depend on the internal structure of a specific repository.

## Development commands

```sh
# during development
gofmt -l . # list unformatted files (CI fails on any output)
gofmt -w . # apply formatting
go vet ./...
go test ./...
go build -o /tmp/list-versions ./cmd/list-versions

# cross-build (Windows sanity check)
GOOS=windows GOARCH=amd64 \
    go build -o /tmp/list-versions.exe ./cmd/list-versions

# with version embedded
go build -ldflags "-X main.version=vX.Y.Z" \
    -o /tmp/list-versions ./cmd/list-versions
```

## Release procedure

1. `git tag vX.Y.Z`
2. `git push origin vX.Y.Z`
3. `.github/workflows/release.yml` triggers GoReleaser.
   It posts the per-OS/arch binaries, checksums and changelog to the Release.
4. Confirm that it installs with
   `go install github.com/KazuyaMiyagi/list-versions/cmd/list-versions@vX.Y.Z`

For known limitations, see the Limitations section in `README.md`.
