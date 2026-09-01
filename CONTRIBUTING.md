# Contributing to gdu

Thanks for your interest in contributing. This guide describes how the project
is developed so your change fits in with the rest of the codebase.

## Prerequisites

- **Go** 1.26 or newer (development happens on 1.27; CI runs 1.26.x and 1.27.x). The pinned toolchain is in `.tool-versions`.
- **Node.js** — only needed if you change the web UI (`webui/frontend`). The
  compiled assets in `webui/dist` are committed and embedded into the binary, so
  pure-Go builds don't require Node.

Install the development tools (`gotestsum`, `gox`, `golangci-lint`, ...) with:

```sh
make install-dev-dependencies
```

## Project layout

- `cmd/gdu` — CLI entrypoint and flag wiring.
- `pkg/` — reusable libraries: `analyze`, `device`, `fs`, `path`, `remove`,
  `timefilter`, `annex`.
- `stdout/`, `report/`, `tui/` — output backends (non-interactive stdout, JSON
  export/import, and the interactive terminal UI).
- `webui/` — the optional React web UI (compiled output is committed).
- `docs/`, `configuration.md`, `INSTALL.md`, `gdu.1.md` — documentation. The man
  page `gdu.1` is generated from `gdu.1.md`.

## Building and running

```sh
make run          # go run the CLI
make build        # build a binary into dist/
make build-web    # rebuild the web UI assets (requires Node.js)
```

## Testing

Tests are colocated with the code as `*_test.go`. Platform-specific tests use
suffixes such as `*_linux_test.go` and `*_windows.go`; keep OS-specific behavior
behind the matching build constraints.

```sh
make test              # gotestsum
make coverage          # race detector + coverage profile
make coverage-html     # open the coverage report in a browser
go test ./...          # plain go test, no extra tooling
```

CI runs the full suite on Go 1.26.x and 1.27.x (Ubuntu, plus macOS on 1.27.x),
with the race detector, and uploads coverage to Codecov. Coverage upload is set
to fail the build on error, so add tests for new behavior and keep existing
tests passing.

## Linting and formatting

The project uses `golangci-lint` (v2) with a strict configuration in
`.golangci.yml`. Formatting is enforced via `gofmt` and `goimports`.

```sh
make lint    # golangci-lint run -c .golangci.yml
gofmt -w .   # format
```

Run `make lint` before opening a pull request — the same check runs in CI and
must pass.

## Commit messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/).
Prefix each commit subject with a type, optionally scoped:

```
<type>[optional scope]: <description>
```

Common types used in this repo:

- `feat:` — a new feature
- `fix:` — a bug fix
- `perf:` — a performance improvement
- `refactor:` — a code change that neither fixes a bug nor adds a feature
- `docs:` — documentation only
- `test:` — adding or fixing tests
- `chore:` — tooling, dependencies, and maintenance (e.g. `chore(deps):`)

Examples drawn from the history:

```
feat: stop scanning and keep partial results
fix(report): make JSON export respect -t, --depth and --summarize filters
perf: concurrent processing in sqlite analyzer
chore(deps): bump modernc.org/sqlite from 1.49.1 to 1.51.0
```

Keep the description in the imperative mood and lower case, with no trailing
period.

## Pull requests

- Branch off `master` and open your pull request against `master`.
- Pull requests are **squash-merged**, and the final commit subject becomes the
  release changelog entry — so make it a clean Conventional Commit. GitHub
  appends the PR number automatically (e.g. `feat: add web UI (#617)`).
- Make sure `make lint` and the test suite pass locally before requesting
  review.
- Add tests and update documentation (`configuration.md`, `gdu.1.md`, etc.) when
  your change affects behavior or flags.
- Keep the change focused; unrelated refactors belong in separate PRs.

## Reporting issues

Use the GitHub issue templates for
[bug reports](.github/ISSUE_TEMPLATE/bug_report.md) and
[feature requests](.github/ISSUE_TEMPLATE/feature_request.md). Include your gdu
version, OS, and reproduction steps for bugs.
