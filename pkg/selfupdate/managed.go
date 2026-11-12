package selfupdate

import "strings"

// managedParts are locations owned by package managers. Builds made by package
// managers cannot update themselves (see build.SelfUpdate), but some of them,
// e.g. scoop, winget or binenv, install the release binaries as they are.
//
// System directories such as /usr/bin are not listed: a release binary is
// installed there by hand, as the README suggests.
var managedParts = []string{
	"/cellar/", "/homebrew/", "/linuxbrew/", "/snap/", "/nix/store/", "conda",
	"/scoop/", "/winget/", "/chocolatey/",
	"/.binenv/", "/.x-cmd", "/mise/installs/", "/.asdf/installs/", "/aquaproj-aqua/",
}

// isManagedPath reports whether the binary at path looks like it was installed
// by a package manager.
func isManagedPath(path string) bool {
	p := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))

	for _, part := range managedParts {
		if strings.Contains(p, part) {
			return true
		}
	}
	return false
}
