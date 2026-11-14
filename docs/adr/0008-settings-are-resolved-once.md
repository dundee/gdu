# 8. Settings are resolved once

Date: 2026-11-14

## Status

Accepted

## Context

The command line and the config files fill `app.Flags`, about 65 fields
which are also the schema of the config file. Until now `app.Run` turned
them into the behaviour of a run in many places:

- **Flags were mutated while running.** `--no-cross` appended the mount
  points to `IgnoreDirs`, and `main` forced `ShowApparentSize` on Windows.
  `--write-config` then saved values the user never set.
- **Frontends were configured five ways:**
  - positional constructor arguments (`stdout` took 12, seven of them
    adjacent booleans);
  - about 30 functional options for the TUI;
  - 16 setters on the `app.UI` interface;
  - frontend-specific setters;
  - fields of the embedded `common.UI`.
- **The analyzer was chosen in three places.** stdout picked the top-dir
  analyzer itself, and `app.Run` then replaced it, or the default one, for
  `--db` or `--sequential`.
- **Contradictory flags were resolved silently.** `--sequential` replaced
  the `--db` analyzer, so nothing was stored; `--web` ignored `-o` and `-d`;
  `-f` won over `-r`; `-d` won over both. Other contradictions failed only
  inside a frontend, after setting up.
- **A broken config file was ignored without a trace.** The error went to
  the log, which defaults to `/dev/null`.

## Decision

- `cmd/gdu/app` resolves the flags once, at the start of `Run`, into
  `config.Settings` (`internal/config`). They are grouped by concern: `Scan`,
  `Display`, `Delete`, `TUI` (with its `Style`), `Export` and `Web`.
  Resolving computes what depends on the run (the mount points for
  `--no-cross`, the ignore patterns of all their sources, the time filter,
  colours and progress for the frontend and terminal) and leaves `Flags` as
  they were.
- Flags which contradict each other are refused with an error naming both,
  before anything is set up.
- One function chooses the analyzer: the database of `--db`, a sequential
  one for `--sequential`, the top-dir analyzer for stdout without `--top` or
  `--depth`, and otherwise a parallel one.
- Each frontend is created from the analyzer and the settings, and is not
  configured afterwards. `common.UI` is built from them (`common.NewUI`).
  `app.UI` keeps only what a frontend does: list devices, analyze one or
  several paths, read an analysis or a database, and run.
- A config file that cannot be read is reported on stderr, naming the file.
  gdu still runs, as before.
- Creating a frontend changes no process-wide state:
  - its colours and spinner are its own (no `color.NoColor`, no shared
    progress runes);
  - the TUI sets tview's styles, which only tview reads;
  - the profiling server has its own mux instead of replacing
    `http.DefaultServeMux`;
  - `--max-cores` is applied by the gdu command, not by `App.Run`, so a
    program embedding gdu keeps its own `GOMAXPROCS`.

`Flags` and `App.Run` stay the public API of `cmd/gdu/app`. `config.Settings`
is internal, so other programs cannot name it.

## Consequences

- A setting has one way in. A frontend's tests name the settings they need
  in a literal, instead of positional booleans followed by setters.
- `--write-config` saves only what the user configured.
- A run which contradicts itself now fails where it used to pick one of the
  flags. This also applies when one flag comes from the config file, such as
  `sequential-scanning: true` with `--db` on the command line.
- The ignore patterns, the time filter and the cleanup rules are checked
  before anything is set up.
- Tests of the frontends no longer depend on their order through process
  globals, except tview's styles.
