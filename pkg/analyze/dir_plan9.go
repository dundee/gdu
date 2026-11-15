package analyze

import (
	"os"
)

// fileStats returns the apparent size as the disk usage: Plan9 exposes no
// block allocation. Hard links are not identified.
func fileStats(info os.FileInfo, _ string) (usage int64, link inodeKey) {
	return info.Size(), link
}

// dirOwnUsage returns 0: Plan9 exposes no block allocation for a
// directory, so gdu charges it nothing rather than guessing a block.
func dirOwnUsage(string) int64 {
	return 0
}

// dirInfoOwnUsage is dirOwnUsage for a directory that has already been stat'ed.
func dirInfoOwnUsage(os.FileInfo) int64 {
	return 0
}
