package tui

import (
	"testing"

	"github.com/dundee/gdu/internal/testanalyze"
	"github.com/dundee/gdu/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeByApparentSize(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "ddd")
}

func TestSortByApparentSizeAsc(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortOrder = "asc"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "aaa")
}

func TestAnalyzeBySize(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, false)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "ddd")
}

func TestSortBySizeAsc(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, false)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortOrder = "asc"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "aaa")
}

func TestAnalyzeByName(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortBy = "name"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "aaa")
}

func TestAnalyzeByNameAsc(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortBy = "name"
	ui.sortOrder = "asc"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "ddd")
}

func TestAnalyzeByItemCount(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortBy = "itemCount"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "ddd")
}

func TestAnalyzeByItemCountAsc(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortBy = "itemCount"
	ui.sortOrder = "asc"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(4, 0).Text, "aaa")
}

func TestSetSorting(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = testanalyze.MockedProcessDir
	ui.done = make(chan struct{})
	ui.sortBy = "itemCount"
	ui.sortOrder = "asc"
	ui.AnalyzePath("test_dir", nil)

	<-ui.done

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.setSorting("name")
	assert.Equal(t, "name", ui.sortBy)
	assert.Equal(t, "asc", ui.sortOrder)
	ui.setSorting("name")
	assert.Equal(t, "name", ui.sortBy)
	assert.Equal(t, "desc", ui.sortOrder)
	ui.setSorting("name")
	assert.Equal(t, "name", ui.sortBy)
	assert.Equal(t, "asc", ui.sortOrder)
}
