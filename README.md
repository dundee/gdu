# gdu

[![Build Status](https://travis-ci.com/dundee/gdu.svg?branch=master)](https://travis-ci.com/dundee/gdu)
[![codecov](https://codecov.io/gh/dundee/gdu/branch/master/graph/badge.svg)](https://codecov.io/gh/dundee/gdu)
[![Go Report Card](https://goreportcard.com/badge/github.com/dundee/gdu)](https://goreportcard.com/report/github.com/dundee/gdu)

Pretty fast disk usage analyzer written in Go.
Inspired by [ncdu](https://dev.yorhel.nl/ncdu), [godu](https://github.com/viktomas/godu) and [df](https://www.gnu.org/software/coreutils/manual/html_node/df-invocation.html).

<img src="/assets/demo.gif" width="100%" />

## Installation

Go:

    go get -u github.com/dundee/gdu


Arch Linux:

    yay -S gdu


## Usage

    gdu                                 # show all mounted disks
    gdu some_dir_to_analyze             # analyze given dir
    gdu -log-file=./gdu.log some_dir    # write errors to log file
    gdu -ignore=/sys,/proc /            # ignore some paths


## Running tests

    make test


## Benchmark

Scanning 80G of data on 500 GB SSD.

Tool       | Without cache | With cache
 ---       | ---           | --- 
gdu /      | 6.5s          | 2.5s
godu /     | 8.5s          | 3s
du -hs /   | 44s           | 4.5s
ncdu /     | 54s           | 12s
