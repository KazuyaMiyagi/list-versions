# Design decision log

A record of "why it's done this way" for list-versions. For implementation
conventions and procedures see `CLAUDE.md`; for the user-facing description see
`README.md`. This is a rationale/trade-off document (ADR-style), so there's no
need to read it every time.

## Why `--deps` is opt-in

This is the area where Dependabot files per-package PRs, so mixing it into the default output would explode the row count and bury the main purpose (a runtime/IaC overview). Because "where to draw the line between framework and utility is fuzzy," the policy is: if opt-in, emit **all direct dependencies at once**.

## Why lockfiles are also read

`gem "rails", "~> 7.0"` in a `Gemfile` only yields a **constraint**, so the only way to know the **actual version** is to look at `Gemfile.lock`. By emitting both the constraint and the actual version on **separate rows**, discrepancies between the two become easy to spot.

Lockfiles intentionally emit **every resolved package — transitive included**, not just the direct ones declared in the manifest (this holds for `Gemfile.lock`, `Cargo.lock`, `poetry.lock`, `uv.lock`, and `package-lock.json` for both the v1 and v2/v3 formats). Transitive packages are real, installed code, and a vulnerable pinned version is just as likely to be a transitive dependency — so the full installed set is what makes the inventory useful for "which version is this, anywhere in the tree?". The manifest managers stay direct-only; the lockfile rows are where the complete picture lives.

## Why focus on Lambda runtime (the LTS-only policy)

AWS Lambda **only offers versions that have been promoted to Active LTS**. For Node.js that's even-numbered releases only (22, 24, 26); odd-numbered Current releases (21, 23, 25) are skipped. Setting a local `.node-version` to the latest Current makes it undeployable to Lambda, so there's a lot of value in `list-versions` showing, in one vertical view, whether the Node major version is out of sync across local / Lambda / Dockerfile / GitHub Actions.

## Why no external API calls at runtime

It runs reliably in CI / works fully offline / no rate limits to worry about / no cache layer needed. EOL data is likewise intended to be embedded at build time via `go:embed` (the Trivy approach).

## Why the sort key is linked to the column order

It follows the natural mental model of "leftmost = primary sort." `-s type,version` produces a TYPE-first grouped view in one shot.

## Why the TYPE column isn't collapsed into one

The row key is the tuple `(Type, Path, Name)`. Reality includes cases like `Rails 7.0 / 7.1 / 7.2` where the **same Type has different versions per location**, so it's **one row per location**, not one row per TYPE (aggregated). If you want aggregation, exporting to CSV and pivoting in a spreadsheet is more flexible.

## Why root paths are kept as-is (not made cwd-relative)

It follows the Unix conventions of `find` / `grep -r` / `rg` / `fd`. With `list-versions ../infra`, having the DIRECTORY column start with `../infra/...` is more predictable, and you can copy-paste it directly, e.g. `cat ../infra/...`.

## Why SARIF isn't implemented

There's a theoretical use for emitting EOL-based findings to Code Scanning, but it's unnecessary for the current primary use case (inventory → paste into a spreadsheet). Reconsider once EOL is implemented and user demand appears.

## Why there's no config file

The tool is small, and command-line flags alone are enough. Enabling/disabling Managers is fine with everything ON (toggle only big branches like `--deps` via a flag, and filter output with `--type`). Adding a config file snowballs into deciding "project-local vs global config," "precedence," "format selection," and so on, so the policy is to keep everything in CLI flags.

## Why there's no cache

The design never calls external APIs at runtime in the first place (the EOL DB is managed offline), so a cache layer is unnecessary. Managers walk the tree every time, but for repos of the target size that finishes in milliseconds.

## Flags follow the standard Unix style

Each flag has a short (`-d`) and long (`--deps`) form. Short forms are **all lowercase** (`-f` / `-s` / `-t` / `-d` / `-x` / `-v` / `-h`). We don't adopt the `-V = version, -v = verbose` convention; since this CLI has no verbose mode, `-v = version` (matching the npm / node / ruby intuition).
