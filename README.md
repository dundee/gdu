# gdu

[![Build Status](https://travis-ci.com/dundee/gdu.svg?branch=master)](https://travis-ci.com/dundee/gdu)
[![codecov](https://codecov.io/gh/dundee/gdu/branch/master/graph/badge.svg)](https://codecov.io/gh/dundee/gdu)
[![Go Report Card](https://goreportcard.com/badge/github.com/dundee/gdu)](https://goreportcard.com/report/github.com/dundee/gdu)

Extremely fast disk usage analyzer.
Port of [ncdu](https://dev.yorhel.nl/ncdu) written in Go with additional abilities of `df`.

<img src="/assets/demo.gif" width="100%" />

## Installation

Go:

    go get -u github.com/dundee/gdu


Arch Linux:

    yay -S gdu


## Usage

    gdu                      # Show all mounted disks
    gdu some_dir_to_analyze  # analyze given dir


## Running tests

    make test
