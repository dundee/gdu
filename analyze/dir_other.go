// +build windows plan9

package analyze

import (
	"os"
)

func getUsage(f os.FileInfo) int64 {
	return int64(0)
}
