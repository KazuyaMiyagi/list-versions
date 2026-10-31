# list-versions

A version-declaration inventory CLI for getting
a "bird's-eye view when planning upgrades."

It lists the areas Dependabot doesn't touch in a single command.
These are language runtimes, IaC resources, CI tools and managed cloud services.
Application-level direct dependencies are also supported,
opt-in via `--deps`.

## Install

```sh
go install github.com/KazuyaMiyagi/list-versions/cmd/list-versions@latest
```

For a Brewfile:

```ruby
go "github.com/KazuyaMiyagi/list-versions/cmd/list-versions"
```

## Usage

```sh
# list everything under the current directory
list-versions

# pass multiple paths (expanded by the shell glob)
list-versions ~/src/github.com/*/

# emit as CSV or JSON
list-versions --format csv
list-versions --format json

# filter by TYPE (glob supported)
list-versions --type Ruby
list-versions --type 'Lambda,RDS,Aurora'
list-versions --type 'gem:rails*'

# change the sort keys (column order follows)
list-versions --sort type,version,directory,file

# include application-level direct dependencies and lockfiles
list-versions --deps

# add directories to exclude
list-versions --exclude .venv,tmp
```

## Detected sources

### Language runtimes and tool versions

- `.python-version`/`.ruby-version`/`.node-version`/`.terraform-version`/
  `.go-version`/`.openapi-generator-version`/`.bun-version`,
  and the `.X-version` family in general
- `.nvmrc`/`.tool-versions` (asdf/mise)
- `package.json` `engines.{node,npm,pnpm,yarn,bun}`
  and `packageManager` (corepack)
- the `ruby "..."` directive in `Gemfile`/`gems.rb`
- `BUNDLED WITH` in `Gemfile.lock`/`gems.locked` (Bundler)
- `required_ruby_version` and `required_rubygems_version` (RubyGems)
  in `*.gemspec`
- `requires-python`/Poetry's `python` in `pyproject.toml`
- `[tool.poetry] requires-poetry` and `[tool.uv] required-version`
  in `pyproject.toml`
- `required-version` in `uv.toml`
- the Poetry version in the `@generated` header of `poetry.lock`

### Containers

- `FROM image:tag[@sha256:...]` in `Dockerfile`/`Dockerfile.*`
  (`ARG NAME=default` is also resolved)
- Kubernetes manifests: container and initContainer images on
  - `Pod`/`Deployment`/`StatefulSet`/`DaemonSet`
  - `ReplicaSet`/`Job`/`CronJob`
- Helm Chart `Chart.yaml` (`version`/`appVersion`)
  and `image.tag` in `values.yaml`

### Terraform

- `terraform { required_version }`/`required_providers`/module `version`
- AWS resource attributes:
  - Lambda runtime:
    - `aws_lambda_function.runtime`
    - `aws_lambda_layer_version.compatible_runtimes`
  - RDS/Aurora
    (`aws_db_instance.engine_version`, `aws_rds_cluster.engine_version`)
  - ElastiCache (`aws_elasticache_*` Redis/Memcached/Valkey)
  - OpenSearch (`aws_opensearch_domain.engine_version`)
  - EKS (`aws_eks_cluster.version`, `aws_eks_node_group.release_version`)
  - ECS (`aws_ecs_task_definition.container_definitions`)
  - App Runner
    (`aws_apprunner_service.source_configuration.image_repository.image_identifier`)
  - AMI (`aws_instance.ami`, `aws_launch_template.image_id`)
- GCP resource attributes:
  - Cloud Run v2 (Service/Job/Gen1) container image
  - Cloud Functions:
    - `google_cloudfunctions_function`
    - `google_cloudfunctions2_function.build_config.runtime`
  - App Engine (`google_app_engine_*_app_version.runtime`)
  - Cloud SQL (`google_sql_database_instance.database_version`)
  - Compute (`google_compute_instance.boot_disk.initialize_params.image`)

### CloudFormation

- version properties in YAML or JSON templates:
  - `Resources.*.Properties.{Runtime, EngineVersion, Version, ReleaseVersion, ...}`
  - Lambda/SAM/RDS/Aurora/ElastiCache/OpenSearch/EKS

### Serverless Framework

- `provider.runtime`/`functions.*.runtime` in `serverless.yml`

### CI

- in `.github/workflows/*.{yml,yaml}`:
  - `uses: foo/bar@ref` (the `# v4.2.2` comment on a SHA pin is preserved)
  - `with: *-version`/`*-version-file`/the `version` of setup-style actions
  - `${{ env.X }}`/`${{ matrix.X }}` resolution
    (matrix is expanded into rows as a cross product)

### Application dependencies (opt-in via `--deps`)

<!-- markdownlint-disable MD013 -->

| Ecosystem | Manifest                                                        | Lockfile                                                    |
| --------- | --------------------------------------------------------------- | ----------------------------------------------------------- |
| Ruby      | `Gemfile`/`gems.rb`/`*.gemspec`                                 | `Gemfile.lock`/`gems.locked`                                |
| npm       | `package.json` (deps, devDeps)                                  | `package-lock.json`/`yarn.lock`/`pnpm-lock.yaml`/`bun.lock` |
| Python    | `pyproject.toml` (PEP 621, Poetry)/`Pipfile`/`requirements.txt` | `poetry.lock`/`Pipfile.lock`/`uv.lock`                      |
| Rust      | `Cargo.toml`                                                    | `Cargo.lock`                                                |
| Go        | —                                                               | `go.mod` (actual versions, `// indirect` excluded)          |
| PHP       | `composer.json`                                                 | `composer.lock`                                             |
| Elixir    | `mix.exs`                                                       | `mix.lock`                                                  |

<!-- markdownlint-enable MD013 -->

## Output

The default is a table:

```text
┌──────────────┬───────────────┬─────────┬─────────┐
│ DIRECTORY    │ FILE          │ TYPE    │ VERSION │
├──────────────┼───────────────┼─────────┼─────────┤
│ myapp        │ .ruby-version │ Ruby    │ 3.4.9   │
│ myapp        │ .node-version │ Node.js │ 24.13.0 │
│ myapp/worker │ .node-version │ Node.js │ 25.9.0  │
└──────────────┴───────────────┴─────────┴─────────┘
```

`--format csv` and `--format json` produce the other formats.
With `--sort`, the column order and sort keys move together.

## Limitations

- Bun's binary lockfile (`bun.lockb`) is unsupported.
  Only `bun.lock` (text) is supported.
- CloudFormation short-form intrinsics (`!Ref X`, etc.) leak through as values.
  yaml.v3 discards their tag information.
  The long form `Ref: X` is skipped correctly.
- Helm `values.yaml` is only read for a simple top-level `image.tag`.
  Sub-charts and multiple images are unsupported.
- Env set via `$GITHUB_ENV` (`echo X=Y >> $GITHUB_ENV`) is left as an expression.
  It can't be analyzed statically.
- Terraform `Lambda@Edge` detection only works within one module directory.
  Cross-module setups are unsupported.
- In `*.gemspec`, only string literals on the same line are read.
  Constants and `#{}` interpolation are skipped.

## Release

Releases are driven by pushing a semver tag.
GitHub Actions (`.github/workflows/release.yml`) runs GoReleaser.
GoReleaser creates the per-OS/arch binaries and the GitHub Release.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Once the tag is pushed to GitHub, the following are attached to the Release:

- binary archives for `darwin/amd64`, `darwin/arm64`, `linux/amd64`,
  `linux/arm64`, `windows/amd64` and `windows/arm64`
- `checksums.txt`
- a changelog (Conventional Commits based)

Users can also just install it via `go install ...@vX.Y.Z`.

## License

MIT
