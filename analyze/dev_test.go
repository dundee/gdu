package analyze

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicesInfo(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	devices := GetDevicesInfo()
	assert.NotEmpty(t, devices)
}
