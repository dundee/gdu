package tui

import (
	"testing"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/internal/testapp"
	"github.com/dundee/gdu/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestSortBySizeAsc(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)

	ui := CreateUI(app, true, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

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
	fin := testdir.CreateTestDir()
	defer fin()

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)

	ui := CreateUI(app, false, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

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
	fin := testdir.CreateTestDir()
	defer fin()

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)

	ui := CreateUI(app, false, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

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
	fin := testdir.CreateTestDir()
	defer fin()

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)

	ui := CreateUI(app, true, false)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

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
	fin := testdir.CreateTestDir()
	defer fin()

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)

	ui := CreateUI(app, false, true)

	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

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
