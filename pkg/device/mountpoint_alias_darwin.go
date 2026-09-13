package device

import "strings"

// dataVolumePath is where macOS mounts the APFS Data volume. Firmlinked
// directories such as /Volumes are reachable both from / and from below it.
const dataVolumePath = "/System/Volumes/Data"

// getMountPointAlias returns the path of mount point as seen through the Data volume,
// e.g. /System/Volumes/Data/Volumes/USB for /Volumes/USB.
func getMountPointAlias(mountPoint string) (string, bool) {
	if mountPoint == "/" || mountPoint == dataVolumePath ||
		strings.HasPrefix(mountPoint, dataVolumePath+"/") {
		return "", false
	}
	return dataVolumePath + mountPoint, true
}
