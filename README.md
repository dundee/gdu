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


## Running tests

    make test
