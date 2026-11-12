# 4. Only release binaries update themselves

Date: 2026-11-12

## Status

Accepted

## Context

Users who download gdu from the GitHub releases page have no convenient way to
update it. Tools such as Outreach's `orc` (through gobox's updater) update
themselves on every run: they check GitHub, replace the binary and re-run the
command.

Most gdu users, however, install it through a package manager: apt, Homebrew,
snap, winget, scoop, AUR, COPR, `go install` and others. A binary that replaces
itself would fight the package manager, which then no longer knows what is
installed. Some of these (scoop, winget, binenv) install the release binaries
unchanged, so the binary alone cannot tell how it got onto the system.

gdu also does not access the network, and a disk usage analyzer has no reason to
start doing so on every run.

## Decision

- `--self-update` is an explicit flag, there is no automatic check. A flag
  rather than a subcommand, because every positional argument is a directory to
  scan.
- It works only when `build.SelfUpdate` is `"true"`, which `make build-all` sets
  for the binaries attached to GitHub releases. Package managers build from
  source without it, so their builds never replace themselves without any action
  from packagers. The version must also be a clean `vX.Y.Z`.
- As a second line of defence, binaries whose resolved path lies in a known
  package-manager location (Homebrew Cellar, snap, nix store, scoop, winget,
  binenv, mise, asdf, aqua, ...) refuse to update. System directories such as
  `/usr/bin` are not on the list, because the README installs the release binary
  there by hand.
- The archive is verified against the published `sha256sums.txt` before the
  binary is replaced. The GPG signature of that file is not checked yet.
- The binary is swapped atomically next to the old one (`fynelabs/selfupdate`,
  which also handles Windows, where a running binary cannot be overwritten).
- When `--self-update` is the only argument gdu exits after updating. Otherwise
  the new binary is run with the remaining arguments (`exec` on Unix, a child
  process on Windows). If the update fails or there is nothing to update, the
  command runs with the current binary.

## Consequences

- Users of release binaries can update with `gdu --self-update`, or `sudo gdu
  --self-update` when the binary is not writable by them.
- Package-manager installs are unaffected and get a message pointing them at
  their package manager.
- Checksum verification detects corrupted downloads but not a compromised
  release, the same trust users put in the releases page today.
- Release assets must keep their names (`gdu_<os>_<arch>[...]`.tgz/.zip) and the
  `sha256sums.txt` file, otherwise older binaries cannot find their update.
