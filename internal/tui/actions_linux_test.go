//go:build linux

package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/config"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
)

func TestShowDevicesWithError(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}
