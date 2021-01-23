package tui

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestFooter(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	ui := CreateUI(simScreen, false)

	dir := analyze.File{
		Name:      "xxx",
		BasePath:  ".",
		Size:      5,
		ItemCount: 2,
	}

	file := analyze.File{
		Name:      "yyy",
		Size:      2,
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

	text := []byte(" Apparent size: 5 B Items: 2")
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

	ui := CreateUI(simScreen, false)
	progress.CurrentItemName = "xxx"
	ui.updateProgress(progress)
	assert.True(t, true)
}

func TestHelp(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	cells := b[257 : 257+7]

	text := []byte("selected")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestDeleteDir(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestDeleteDirWithConfirm(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
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
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(10 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.FileExists(t, "test_dir/nested/file2")
}

func TestRescan(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true)

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

	ui := CreateUI(simScreen, false)
	ui.ListDevices(getDevicesInfoMock)
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

	ui := CreateUI(simScreen, true)
	ui.ListDevices(getDevicesInfoMock)
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

	ui := CreateUI(simScreen, true)
	ui.SetIgnoreDirPaths([]string{"/"})
	ui.analyzer = analyzeMock
	ui.ListDevices(getDevicesInfoMock)

	go func() {
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(10 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
	}()

	ui.StartUILoop()
}

func TestKeys(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)
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
		simScreen.InjectKey(tcell.KeyEnter, 'h', 1)
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
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)

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

func getDevicesInfoMock(_ string) ([]*analyze.Device, error) {
	item := &analyze.Device{
		Name: "xxx",
	}
	return []*analyze.Device{item}, nil
}
