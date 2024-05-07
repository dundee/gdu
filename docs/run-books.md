# Release process

1. update usage in README.md and gdu.1.md
1. `make show-man`
1. `make man`
1. commit the changes
1. tag new version with `-sa`
1. `make`
1. `git push --tags`
1. `git push`
1. `make release`
1. update `gdu.spec`
1. Release snapcraft, AUR, ...
