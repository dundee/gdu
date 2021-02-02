package tui

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/device"
	"github.com/dundee/gdu/internal/test_dev"
	"github.com/dundee/gdu/internal/test_dir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestFooter(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	ui := CreateUI(simScreen, false, true)

	dir := analyze.File{
		Name:      "xxx",
		BasePath:  ".",
		Size:      5,
		Usage:     4096,
		ItemCount: 2,
	}

	file := analyze.File{
		Name:      "yyy",
		Size:      2,
		Usage:     4096,
		ItemCount: 1,
		Parent:    &dir,
	}
	dir.Files = []*analyze.File{&file}

	ui.currentDir = &dir
	ui.showDir()
	ui.pages.HidePage("progress")

	ui.footer.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte(" Total disk usage: 4.0 KiB Apparent size: 5 B Items: 2")
	for i, r := range b {
		if i >= len(text) {
			break
		}
		assert.Equal(t, string(text[i]), string(r.Bytes[0]))
	}
}

func TestUpdateProgress(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	progress := &analyze.CurrentProgress{Mutex: &sync.Mutex{}, Done: true}

	ui := CreateUI(simScreen, false, false)
	progress.CurrentItemName = "xxx"
	ui.updateProgress(progress)
	assert.True(t, true)
}

func TestHelp(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false, true)
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	cells := b[264 : 264+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestDeleteDir(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, false)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1) // test selecting file
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'a', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestDoNotDeleteParentDir(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, true)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.FileExists(t, "test_dir/nested/file2")
}

func TestDeleteDirWithConfirm(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, 'x', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestShowConfirm(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1) // cannot go up
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRight, '1', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRight, '1', 1) // `..` cannot be selected by `l` or `right`
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1) // select file
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1) // cannot go up when confirm is shown
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1) // cannot go down when confirm is shown
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.FileExists(t, "test_dir/nested/file2")
}

func TestRescan(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		time.Sleep(10 * time.Millisecond)

		// rescan subdir
		simScreen.InjectKey(tcell.KeyRune, 'r', 1)
		time.Sleep(100 * time.Millisecond)

		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()
}

func TestShowDevices(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false, true)
	ui.ListDevices(getDevicesInfoMock())
	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestShowDevicesBW(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, false)
	ui.ListDevices(getDevicesInfoMock())
	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestSelectDevice(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true, true)
	ui.analyzer = analyzeMock
	ui.SetIgnoreDirPaths([]string{"/proc"})
	ui.ListDevices(getDevicesInfoMock())

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1) // device cannot be deleted
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'r', 1) // or refreshed
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
	}()

	ui.StartUILoop()
}

func TestKeys(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false, false)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/subnested/file")
}

func TestSetIgnoreDirPaths(t *testing.T) {
	fin := test_dir.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false, true)

	path, _ := filepath.Abs("test_dir/nested/subnested")
	ui.SetIgnoreDirPaths([]string{path})

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, 3, dir.ItemCount)

}

func printScreen(simScreen tcell.SimulationScreen) {
	b, _, _ := simScreen.GetContents()

	for i, r := range b {
		println(i, string(r.Bytes))
	}
}

func analyzeMock(path string, progress *analyze.CurrentProgress, ignore analyze.ShouldDirBeIgnored) *analyze.File {
	return &analyze.File{
		Name:     "xxx",
		BasePath: ".",
	}
}

func getDevicesInfoMock() device.DevicesInfoGetter {
	item := &device.Device{
		Name:       "/dev/root",
		MountPoint: "/",
	}
	item2 := &device.Device{
		Name:       "/dev/boot",
		MountPoint: "/boot",
	}

	mock := test_dev.DevicesInfoGetterMock{}
	mock.Devices = []*device.Device{item, item2}
	return mock
}
