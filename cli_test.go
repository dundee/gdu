package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestFooter(t *testing.T) {
	CreateTestDir()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	ui := CreateUI(".", simScreen)

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
	ui.ShowDir()
	ui.pages.HidePage("modal")

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

	statusChannel := make(chan CurrentProgress)

	ui := CreateUI(".", simScreen)
	go func() {
		ui.updateProgress(statusChannel)
	}()

	statusChannel <- CurrentProgress{
		currentItemName: "xxx",
		done:            true,
	}
	assert.True(t, true)
}
