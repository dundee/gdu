package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
	CreateTestDir()

	simScreen := tcell.NewSimulationScreen("UTF-8")
	defer simScreen.Fini()
	simScreen.Init()
	simScreen.SetSize(15, 15)

	ui := CreateUI(".", simScreen)

	ui.currentDir = &File{
		name:      "xxx",
		size:      5,
		itemCount: 2,
	}
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
