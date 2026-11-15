package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
)

// mountsGetter lists devices, and more mounts than devices, as the system
// does: tmpfs, overlay or FUSE filesystems are mounted but no device.
type mountsGetter struct {
	devices, mounts device.Devices
}

func (g mountsGetter) GetDevicesInfo() (device.Devices, error) { return g.devices, nil }
func (g mountsGetter) GetMounts() (device.Devices, error)      { return g.mounts, nil }

// Choosing a device leaves out the mount points of the other filesystems
// below it, as they are, and keeps leaving out what the user asked to.
func TestDeviceSelectionLeavesOutOtherMountsAndKeepsUserPatterns(t *testing.T) {
	// as mount points are, with symlinks resolved
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	for _, name := range []string{"c++", "a.b", "axb", "skipped", "kept"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, name), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, name, "f"), []byte("data"), 0o600))
	}
	pattern, err := common.IgnorePattern([]string{".*skipped"}, "", "", nil)
	require.NoError(t, err)

	app := testapp.CreateMockedApp(false)
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(),
		&config.Settings{Scan: config.Scan{IgnoreDirPattern: pattern}})
	ui.done = make(chan struct{})

	rootDevice := &device.Device{Name: "/dev/root", MountPoint: root, Size: 1e9}
	require.NoError(t, ui.ListDevices(mountsGetter{
		devices: device.Devices{rootDevice},
		mounts: device.Devices{
			rootDevice,
			// listed among the mounts only, and not regular expressions
			{Name: "tmpfs", MountPoint: filepath.Join(root, "c++")},
			{Name: "fuse", MountPoint: filepath.Join(root, "a.b")},
		},
	}))

	ui.deviceItemSelected(1, 0)
	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	var names []string
	for entry := range ui.currentDir.GetFiles(fs.SortByName, fs.SortAsc) {
		names = append(names, entry.GetName())
	}
	assert.Equal(t, []string{"axb", "kept"}, names)
}
