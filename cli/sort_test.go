package cli

import (
	"testing"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestSortBySizeAsc(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 's', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, "file2", dir.Files[0].Name)
}

func TestSortByName(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'n', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, "file2", dir.Files[0].Name)
}

func TestSortByNameDesc(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'n', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'n', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, "subnested", dir.Files[0].Name)
}

func TestSortByItemCount(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'c', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, "file2", dir.Files[0].Name)
}

func TestSortByItemCountDesc(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.SetSize(50, 50)

	ui := CreateUI(simScreen, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	go func() {
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'j', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'l', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'c', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'c', 1)
		time.Sleep(100 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', 1)
		time.Sleep(100 * time.Millisecond)
	}()

	ui.StartUILoop()

	dir := ui.currentDir

	assert.Equal(t, "subnested", dir.Files[0].Name)
}
