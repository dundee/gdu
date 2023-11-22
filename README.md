# go DiskUsage()

[![Codecov](https://codecov.io/gh/dundee/gdu/branch/master/graph/badge.svg)](https://codecov.io/gh/dundee/gdu)
[![Go Report Card](https://goreportcard.com/badge/github.com/dundee/gdu)](https://goreportcard.com/report/github.com/dundee/gdu)
[![Maintainability](https://api.codeclimate.com/v1/badges/30d793274607f599e658/maintainability)](https://codeclimate.com/github/dundee/gdu/maintainability)
[![CodeScene Code Health](https://codescene.io/projects/13129/status-badges/code-health)](https://codescene.io/projects/13129)

Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.

[![asciicast](https://asciinema.org/a/382738.svg)](https://asciinema.org/a/382738)

<a href="https://repology.org/project/gdu/versions">
    <img src="https://repology.org/badge/vertical-allrepos/gdu.svg" alt="Packaging status" align="right">
</a>

## Installation

Head for the [releases page](https://github.com/dundee/gdu/releases) and download the binary for your system.

Using curl:

    curl -L https://github.com/dundee/gdu/releases/latest/download/gdu_linux_amd64.tgz | tar xz
    chmod +x gdu_linux_amd64
    mv gdu_linux_amd64 /usr/bin/gdu

[Arch Linux](https://archlinux.org/packages/extra/x86_64/gdu/):

    pacman -S gdu

[Debian](https://packages.debian.org/bullseye/gdu):

    apt install gdu

[Ubuntu](https://launchpad.net/~daniel-milde/+archive/ubuntu/gdu)

    add-apt-repository ppa:daniel-milde/gdu
    apt-get update
    apt-get install gdu

[NixOS](https://search.nixos.org/packages?channel=unstable&show=gdu&query=gdu):

    nix-env -iA nixos.gdu

[Homebrew](https://formulae.brew.sh/formula/gdu):

    brew install -f gdu
    # gdu will be installed as `gdu-go` to avoid conflicts with coreutils
    gdu-go

[Snap](https://snapcraft.io/gdu-disk-usage-analyzer):

    snap install gdu-disk-usage-analyzer
    snap connect gdu-disk-usage-analyzer:mount-observe :mount-observe
    snap connect gdu-disk-usage-analyzer:system-backup :system-backup
    snap alias gdu-disk-usage-analyzer.gdu gdu

[Binenv](https://github.com/devops-works/binenv)

    binenv install gdu

[Go](https://pkg.go.dev/github.com/dundee/gdu):

    go install github.com/dundee/gdu/v5/cmd/gdu@latest

[Winget](https://github.com/microsoft/winget-pkgs/tree/master/manifests/d/dundee/gdu) (for Windows users):

    winget install gdu

You can either run it as `gdu_windows_amd64.exe` or 
* add an alias with `Doskey`.
* add `alias gdu="gdu_windows_amd64.exe"` to your `~/.bashrc` file if using Git Bash to run it as `gdu`.

[Scoop](https://github.com/ScoopInstaller/Main/blob/master/bucket/gdu.json):

    scoop install gdu
### COPR builds
Amazon Linux 2023:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/amazonlinux-2023-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```
EPEL 7:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/epel-7-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```
EPEL 8:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/epel-8-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```
EPEL 9:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/epel-9-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```
Fedora 38:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/fedora-$releasever-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```
Fedora 39:
```
[copr:copr.fedorainfracloud.org:faramirza:gdu]
name=Copr repo for gdu owned by faramirza
baseurl=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/fedora-$releasever-$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://download.copr.fedorainfracloud.org/results/faramirza/gdu/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
```

## Usage

```
  gdu [flags] [directory_to_scan]

Flags:
      --config-file string            Read config from file (default is $HOME/.gdu.yaml)
  -g, --const-gc                      Enable memory garbage collection during analysis with constant level set by GOGC
      --enable-profiling              Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/
  -L, --follow-symlinks               Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)
  -h, --help                          help for gdu
  -i, --ignore-dirs strings           Absolute paths to ignore (separated by comma) (default [/proc,/dev,/sys,/run])
  -I, --ignore-dirs-pattern strings   Absolute path patterns to ignore (separated by comma)
  -X, --ignore-from string            Read absolute path patterns to ignore from file
  -f, --input-file string             Import analysis from JSON file
  -l, --log-file string               Path to a logfile (default "/dev/null")
  -m, --max-cores int                 Set max cores that GDU will use. 8 cores available (default 8)
  -c, --no-color                      Do not use colorized output
  -x, --no-cross                      Do not cross filesystem boundaries
  -H, --no-hidden                     Ignore hidden directories (beginning with dot)
      --no-mouse                      Do not use mouse
      --no-prefix                     Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode
  -p, --no-progress                   Do not show progress in non-interactive mode
  -n, --non-interactive               Do not run in interactive mode
  -o, --output-file string            Export all info into file as JSON
  -a, --show-apparent-size            Show apparent size
  -d, --show-disks                    Show all mounted disks
  -B, --show-relative-size            Show relative size
      --si                            Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)
  -s, --summarize                     Show only a total in non-interactive mode
  -v, --version                       Print version
      --write-config                  Write current configuration to file (default is $HOME/.gdu.yaml)

In interactive mode:
  ↑ or k                              Move cursor up
  ↓ or j                              Move cursor down
  → or Enter                          Go to highlighted directory
  ← or h                              Go to parent directory
  d                                   Delete the selected file or directory
  e                                   Empty the selected directory
  n                                   Sort by name
  s                                   Sort by size
  c                                   Show number of items in directory
```

## Examples

    gdu                                   # analyze current dir
    gdu -a                                # show apparent size instead of disk usage
    gdu <some_dir_to_analyze>             # analyze given dir
    gdu -d                                # show all mounted disks
    gdu -l ./gdu.log <some_dir>           # write errors to log file
    gdu -i /sys,/proc /                   # ignore some paths
    gdu -I '.*[abc]+'                     # ignore paths by regular pattern
    gdu -X ignore_file /                  # ignore paths by regular patterns from file
    gdu -c /                              # use only white/gray/black colors

    gdu -n /                              # only print stats, do not start interactive mode
    gdu -np /                             # do not show progress, useful when using its output in a script
    gdu -nps /some/dir                    # show only total usage for given dir
    gdu / > file                          # write stats to file, do not start interactive mode

    gdu -o- / | gzip -c >report.json.gz   # write all info to JSON file for later analysis
    zcat report.json.gz | gdu -f-         # read analysis from file

## Modes

Gdu has three modes: interactive (default), non-interactive and export.

Non-interactive mode is started automatically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag.

Export mode (flag `-o`) outputs all usage data as JSON, which can be later opened using the `-f` flag.

Hard links are counted only once.

## File flags

Files and directories may be prefixed by a one-character
flag with following meaning:

* `!` An error occurred while reading this directory.

* `.` An error occurred while reading a subdirectory, size may be not correct.

* `@` File is symlink or socket.

* `H` Same file was already counted (hard link).

* `e` Directory is empty.

## Configuration file

Gdu can read (and write) YAML configuration file.

`$HOME/.config/gdu/gdu.yaml` and `$HOME/.gdu.yaml` are checked for the presense of the config file by default.

### Examples

* To configure gdu to permanently run in gray-scale color mode:

```
echo "no-color: true" >> ~/.gdu.yaml
```

* To set default sorting in configuration file:

```
sorting:
    by: name // size, name, itemCount, mtime
    order: desc
```

* To configure gdu to set CWD variable when browsing directories:

```
echo "change-cwd: true" >> ~/.gdu.yaml
```

* To save the current configuration

```
gdu --write-config
```

## Styling

There are wast ways how terminals can be colored.
Some gdu primitives (like basic text) addapt to different color schemas, but the selected/highlighted row does not.

If the default look is not sufficient, it can be changed in configuration file, e.g.:

```
style:
    selected-row:
        text-color: black
        background-color: "#ff0000"
```

## Memory usage

### Automatic balancing

Gdu tries to balance performance and memory usage.

When less memory is used by gdu than the total free memory of the host,
then Garbage Collection is disabled during the analysis phase completely to gain maximum speed.

Otherwise GC is enabled.
The more memory is used and the less memory is free, the more often will the GC happen.

### Manual memory usage control

If you want manual control over Garbage Collection, you can use `--const-gc` / `-g` flag.
It will run Garbage Collection during the analysis phase with constant level of aggressiveness.
As a result, the analysis will be about 25% slower and will consume about 30% less memory.
To change the level, you can set the `GOGC` environment variable to specify how often the garbage collection will happen.
Lower value (than 100) means GC will run more often. Higher means less often. Negative number will stop GC.

Example running gdu with constant GC, but not so aggressive as default:

```
GOGC=200 gdu -g /
```

## Running tests

    make install-dev-dependencies
    make test

## Benchmarks

Benchmarks were performed on 50G directory (100k directories, 400k files) on 500 GB SSD using [hyperfine](https://github.com/sharkdp/hyperfine).
See `benchmark` target in [Makefile](Makefile) for more info.

## Profiling

Gdu can collect profiling data when the `--enable-profiling` flag is set.
The data are provided via embedded http server on URL `http://localhost:6060/debug/pprof/`.

You can then use e.g. `go tool pprof -web http://localhost:6060/debug/pprof/heap`
to open the heap profile as SVG image in your web browser.

### Cold cache

Filesystem cache was cleared using `sync; echo 3 | sudo tee /proc/sys/vm/drop_caches`.

| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `gdu -npc ~` | 5.833 ± 0.087 | 5.779 | 6.074 | 1.00 |
| `gdu -gnpc ~` | 5.875 ± 0.035 | 5.841 | 5.963 | 1.01 ± 0.02 |
| `diskus ~` | 5.981 ± 0.030 | 5.930 | 6.025 | 1.03 ± 0.02 |
| `pdu ~` | 6.925 ± 0.145 | 6.859 | 7.336 | 1.19 ± 0.03 |
| `dust -d0 ~` | 7.184 ± 0.015 | 7.151 | 7.202 | 1.23 ± 0.02 |
| `dua ~` | 7.212 ± 0.046 | 7.181 | 7.341 | 1.24 ± 0.02 |
| `du -hs ~` | 27.938 ± 0.159 | 27.644 | 28.176 | 4.79 ± 0.08 |
| `ncdu -0 -o /dev/null ~` | 29.032 ± 0.186 | 28.783 | 29.375 | 4.98 ± 0.08 |
| `duc index ~` | 30.823 ± 4.199 | 28.580 | 39.001 | 5.28 ± 0.72 |

### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `diskus ~` | 462.6 ± 6.4 | 452.6 | 474.7 | 1.00 |
| `pdu ~` | 475.9 ± 5.7 | 467.4 | 487.1 | 1.03 ± 0.02 |
| `dua ~` | 667.1 ± 9.8 | 652.8 | 688.2 | 1.44 ± 0.03 |
| `dust -d0 ~` | 748.5 ± 12.9 | 732.6 | 776.7 | 1.62 ± 0.04 |
| `gdu -npc ~` | 894.2 ± 5.1 | 886.6 | 900.8 | 1.93 ± 0.03 |
| `gdu -gnpc ~` | 1051.8 ± 13.9 | 1031.1 | 1074.0 | 2.27 ± 0.04 |
| `du -hs ~` | 1713.2 ± 9.1 | 1698.6 | 1728.7 | 3.70 ± 0.06 |
| `duc index ~` | 2058.4 ± 7.9 | 2046.5 | 2068.5 | 4.45 ± 0.06 |
| `ncdu -0 -o /dev/null ~` | 2807.2 ± 6.1 | 2802.0 | 2821.6 | 6.07 ± 0.09 |

## Alternatives

* [ncdu](https://dev.yorhel.nl/ncdu) - NCurses based tool written in pure `C` (LTS) or `zig` (Stable)
* [godu](https://github.com/viktomas/godu) - Analyzer with a carousel like user interface
* [dua](https://github.com/Byron/dua-cli) - Tool written in `Rust` with interface similar to gdu (and ncdu)
* [diskus](https://github.com/sharkdp/diskus) - Very simple but very fast tool written in `Rust`
* [duc](https://duc.zevv.nl/) - Collection of tools with many possibilities for inspecting and visualising disk usage
* [dust](https://github.com/bootandy/dust) - Tool written in `Rust` showing tree like structures of disk usage
* [pdu](https://github.com/KSXGitHub/parallel-disk-usage) - Tool written in `Rust` showing tree like structures of disk usage
