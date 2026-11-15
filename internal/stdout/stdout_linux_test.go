//go:build linux

package stdout

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/config"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestShowDevicesWithErr(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}
	ui := CreateStdoutUI(output, analyze.CreateTopDirAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}
