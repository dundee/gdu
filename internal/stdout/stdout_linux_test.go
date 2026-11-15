//go:build linux

package stdout

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/stretchr/testify/assert"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelWarn)
}

func TestShowDevicesWithErr(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}
	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}
