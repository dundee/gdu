package main

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestFooter(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	ui := CreateUI(simScreen)

	dir := File{
		name:      "xxx",
		size:      5,
		itemCount: 2,
	}

	file := File{
		name:      "yyy",
		size:      2,
		itemCount: 1,
		parent:    &dir,
	}
	dir.files = []*File{&file}

	ui.currentDir = &dir
	ui.showDir()
	ui.pages.HidePage("progress")

	ui.footer.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Apparent size: 5 B Items: 2")
	for i, r := range b {
		if i >= len(text) {
			break
		}
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestUpdateProgress(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	progress := &CurrentProgress{mutex: &sync.Mutex{}, done: true}

	ui := CreateUI(simScreen)
	progress.currentItemName = "xxx"
	ui.updateProgress(progress)
	assert.True(t, true)
}

func TestHelp(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen)
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	cells := b[308:315]

	text := []byte("selected")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestDeleteDir(t *testing.T) {
	fin := CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir")

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestShowConfirm(t *testing.T) {
	fin := CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen)

	ui.AnalyzePath("test_dir")

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, '?', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, '1', 1)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.FileExists(t, "test_dir/nested/file2")
}

func TestShowDevices(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen)
	ui.ListDevices()
	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func printScreen(simScreen tcell.SimulationScreen) {
	b, _, _ := simScreen.GetContents()

	for i, r := range b {
		println(i, string(r.Bytes))
	}
}

func TestKeys(t *testing.T) {
	fin := CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen)
	ui.askBeforeDelete = false

	ui.AnalyzePath("test_dir")

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'h', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'd', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyEnter, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	assert.NoFileExists(t, "test_dir/nested/subnested/file")
}
