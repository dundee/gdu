// +build linux

package stdout

import (
	"bytes"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestShowDevicesWithErr(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}
	ui := CreateStdoutUI(output, false, true, false)
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}
