# Design decision log

A record of why list-versions is built the way it is.
Implementation conventions and procedures are in `CLAUDE.md`.
The user-facing description is in `README.md`.
This ADR-style document records rationale and trade-offs.
It doesn't need to be read every time.

## Why `--deps` is opt-in

This is the area where Dependabot already opens per-package PRs.
Mixing it into the default output would explode the row count.
It would also bury the main purpose, which is a runtime and IaC overview.
Emitting only the important ones doesn't work either.
The line between a framework and a utility is too fuzzy to draw.
So when opted in, every direct dependency is emitted.

## Why lockfiles are also read

`gem "rails", "~> 7.0"` in a `Gemfile` only yields a constraint.
The actual version can only be found in `Gemfile.lock`.
The constraint and the actual version are emitted as separate rows.
Seeing them side by side makes a mismatch between the two easy to spot.

Lockfiles intentionally emit every resolved package, including transitive ones.
This applies to `Gemfile.lock`, `Cargo.lock`, `poetry.lock` and `uv.lock`.
It also applies to `package-lock.json` in both the v1 and the v2/v3 formats.
Transitive packages are real code that gets installed.
A vulnerable pinned version is just as likely to sit in a transitive dependency.
Listing the whole installed set is what lets you find a version anywhere.
Manifest managers still emit direct dependencies only.
The complete picture lives in the lockfile rows.

## Why focus on Lambda runtime (the LTS-only policy)

AWS Lambda only offers versions that have been promoted to Active LTS.
For Node.js, only the even-numbered releases like 22 and 24 are offered.
The odd-numbered Current releases, like 23 and 25, never become available.
A local `.node-version` set to the latest Current can't be deployed to Lambda.
`list-versions` shows every Node major version in one vertical view.
A mismatch between local files and Lambda becomes easy to catch.
So does one in a Dockerfile or a GitHub Actions workflow.

## Why no external API calls at runtime

Without external API calls, it runs reliably in CI and works fully offline.
There are no rate limits to worry about, and no cache layer is needed.
EOL data is likewise meant to be embedded at build time via `go:embed`,
as Trivy does.

## Why the sort key is linked to the column order

It follows the natural mental model that the leftmost column sorts first.
`-s type,version` produces a TYPE-first grouped view in one shot.

## Why the TYPE column isn't collapsed into one

The row key is the tuple `(Type, Path, Name)`.
A single tree can contain Rails 7.0, 7.1 and 7.2 in different places.
One row per TYPE would hide which location uses which version.
So the output has one row per location instead.
For an aggregated view, export to CSV and pivot it in a spreadsheet.

## Why root paths are kept as-is (not made cwd-relative)

It follows the Unix conventions of `find`, `grep -r`, `rg` and `fd`.
With `list-versions ../infra`, the DIRECTORY column starts with `../infra/...`.
That is more predictable, and the path can be pasted straight into a command.

## Why SARIF isn't implemented

Emitting EOL-based findings to Code Scanning could be useful in theory.
Today the main use is pasting an inventory into a spreadsheet,
which doesn't need it.
Reconsider once EOL support exists and users ask for it.

## Why there's no configuration file

The tool is small, and command-line flags alone are enough.
There's no need to turn Managers on or off, so all of them are always enabled.
Only a big switch like `--deps` gets a flag, and `--type` filters the output.
A configuration file would snowball into more decisions.
Project-local versus global files, precedence and the format would all need settling.
So everything stays in CLI flags.

## Why there's no cache

The design never calls external APIs at runtime in the first place.
The EOL DB is managed offline, so there is nothing to cache.
Managers walk the tree on every run,
and for repos of the target size that finishes in milliseconds.

## Flags follow the standard Unix style

Each flag has a short form (`-d`) and a long form (`--deps`).
All of the short forms are lowercase.
We don't follow the convention of `-V` for version and `-v` for verbose.
This CLI has no verbose mode, so `-v` means version, as in npm and ruby.
