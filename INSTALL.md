# Installation

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

You might need to restart your terminal.

[Scoop](https://github.com/ScoopInstaller/Main/blob/master/bucket/gdu.json):

    scoop install gdu

[X-cmd](https://www.x-cmd.com/start/)

    x env use gdu

## [COPR builds](https://copr.fedorainfracloud.org/coprs/faramirza/gdu/)
COPR Builds exist for the the following Linux Distros.

[How to enable a CORP Repo](https://docs.pagure.org/copr.copr/how_to_enable_repo.html)

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
