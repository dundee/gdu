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

Arch Linux:

    yay -S gdu

Brew:

    brew tap dundee/taps
    brew install gdu

Snap:

    snap install gdu-disk-usage-analyzer
    snap alias gdu-disk-usage-analyzer.gdu gdu

Go:

    go get -u github.com/dundee/gdu


## Usage

    gdu                                 # show all mounted disks
    gdu some_dir_to_analyze             # analyze given dir
    gdu -log-file=./gdu.log some_dir    # write errors to log file
    gdu -ignore-dir=/sys,/proc /        # ignore some paths
    gdu -no-color /                     # use only white/gray/black colors


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
