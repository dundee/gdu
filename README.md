# gdu - Go Disk Usage

[![Build Status](https://travis-ci.com/dundee/gdu.svg?branch=master)](https://travis-ci.com/dundee/gdu)
[![codecov](https://codecov.io/gh/dundee/gdu/branch/master/graph/badge.svg)](https://codecov.io/gh/dundee/gdu)
[![Go Report Card](https://goreportcard.com/badge/github.com/dundee/gdu)](https://goreportcard.com/report/github.com/dundee/gdu)

Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.

[![asciicast](https://asciinema.org/a/382738.svg)](https://asciinema.org/a/382738)

## Installation

Head for the [releases](https://github.com/dundee/gdu/releases) and download binary for your system.

Using curl:

    curl -L https://github.com/dundee/gdu/releases/latest/download/gdu-linux-amd64.tgz | tar xz
    chmod +x gdu-linux-amd64
    mv gdu-linux-amd64 /usr/bin/gdu

[Arch Linux](https://aur.archlinux.org/packages/gdu/):

    yay -S gdu

Debian:

    dpkg -i gdu_*_all.deb

[NixOS](https://search.nixos.org/packages?channel=unstable&show=gdu&query=gdu):

    nix-env -iA nixos.gdu

[Homebrew](https://formulae.brew.sh/formula/gdu):

    brew install gdu

[Snap](https://snapcraft.io/gdu-disk-usage-analyzer):

    snap install gdu-disk-usage-analyzer
    snap alias gdu-disk-usage-analyzer.gdu gdu

[Go](https://pkg.go.dev/github.com/dundee/gdu):

    go get -u github.com/dundee/gdu


## Usage

    gdu                                   # show all mounted disks
    gdu <some_dir_to_analyze>             # analyze given dir
    gdu -l ./gdu.log <some_dir>           # write errors to log file
    gdu -i /sys,/proc /                   # ignore some paths
    gdu -c /                              # use only white/gray/black colors

    gdu -n /                              # only print stats, do not start interactive mode
    gdu -np /                             # do not show progress either
    gdu / > file                          # write stats to file, do not start interactive mode

Gdu has two modes: interactive (default) and non-interactive.

Non-interactive mode is started automtically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag.

## Running tests

    make test


## Benchmark

Scanning 80G of data on 500 GB SSD.

Tool        | Real time without cache | Real time with cache | CPU time without cache (user + sys)
 ---        | ---                     | ---                  | ---               
gdu /       | 6.5                     | 2                    | 15   (8 + 7)
dua /       | 7.5                     | 2                    | 17   (4 + 13)
godu /      | 8                       | 3                    | 23   (11 + 12)
nnn -T d /  | 31                      | 3                    | 7.2  (0.3 + 6.9)
du -hs /    | 32                      | 4                    | 8.6  (0.9 + 7.7)
duc index / | 34                      | 4.5                  | 11.3 (2.5 + 8.8)
baobab /    | 38                      | 12                   | 25   (16 + 9)
ncdu /      | 43                      | 13                   | 18.5 (1.5 + 17)

Gdu is inspired by [ncdu](https://dev.yorhel.nl/ncdu), [godu](https://github.com/viktomas/godu), [dua](https://github.com/Byron/dua-cli) and [df](https://www.gnu.org/software/coreutils/manual/html_node/df-invocation.html).