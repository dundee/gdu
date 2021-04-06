---
date: Jan 2021
section: 1
title: gdu
---

# NAME

gdu - Pretty fast disk usage analyzer written in Go

# SYNOPSIS

**gdu \[flags\] \[directory_to_scan\]**

# DESCRIPTION

Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize
parallel processing. However HDDs work as well, but the performance gain
is not so huge.

# OPTIONS

**-h**, **\--help**\[=false\] help for gdu

**-i**, **\--ignore-dirs**=\[/proc,/dev,/sys,/run\] Absolute paths to
ignore (separated by comma)

**-l**, **\--log-file**=\"/dev/null\" Path to a logfile

**-m**, **\--max-cores** Set max cores that GDU will use.

**-c**, **\--no-color**\[=false\] Do not use colorized output

**-x**, **\--no-cross**\[=false\] Do not cross filesystem boundaries

**-p**, **\--no-progress**\[=false\] Do not show progress in
non-interactive mode

**-n**, **\--non-interactive**\[=false\] Do not run in interactive mode

**-d**, **\--show-disks**\[=false\] Show all mounted disks

**-a**, **\--show-apparent-size**\[=false\] Show apparent size

**-v**, **\--version**\[=false\] Print version

# FILE FLAGS

Files and directories may be prefixed by a one-character
flag with following meaning:

**!**

:   An error occurred while reading this directory.

**.**

:   An error occurred while reading a subdirectory, size may be not correct.

**\@**

:  File is symlink or socket.

**H**

:  Same file was already counted (hard link).

**e**

:  Directory is empty.