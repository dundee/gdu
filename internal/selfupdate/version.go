package selfupdate

import (
	"errors"
	"regexp"
	"strconv"
)

var versionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// version is a release version. Pre-releases and builds between tags
// (git describe output such as v5.39.0-3-gabc1234) are not releases.
type version [3]int

func parseVersion(s string) (version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, errors.New("not a release version")
	}

	var v version
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return version{}, err
		}
		v[i] = n
	}
	return v, nil
}

func (v version) newerThan(other version) bool {
	for i := range v {
		if v[i] != other[i] {
			return v[i] > other[i]
		}
	}
	return false
}
