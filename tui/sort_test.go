package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeByApparentSize(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "desc", true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestSortByApparentSizeAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "asc", true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
}

func TestAnalyzeBySize(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "desc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestSortBySizeAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "asc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ccc")
}

func TestAnalyzeByName(t *testing.T) {
	ui := getAnalyzedPathWithSorting("name", "desc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "aaa")
}

func TestAnalyzeByNameAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("name", "asc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestAnalyzeByItemCount(t *testing.T) {
	ui := getAnalyzedPathWithSorting("itemCount", "desc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "aaa")
}

func TestAnalyzeByItemCountAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("itemCount", "asc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestAnalyzeByMtime(t *testing.T) {
	ui := getAnalyzedPathWithSorting("mtime", "desc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestAnalyzeByMtimeAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("mtime", "asc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "aaa")
}

func TestSetSorting(t *testing.T) {
	ui := getAnalyzedPathWithSorting("itemCount", "asc", false)

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

func TestSetDEfaultSorting(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	var opts []Option
	opts = append(opts, func(ui *UI) {
		ui.SetDefaultSorting("name", "asc")
	})

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false, opts...)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})

	if err := ui.AnalyzePath("test_dir", nil); err != nil {
		panic(err)
	}

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "name", ui.sortBy)
	assert.Equal(t, "asc", ui.sortOrder)
}

func TestSortDevicesByName(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, true, false, false)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)

	ui.setSorting("name") // sort by name asc
	assert.Equal(t, "/dev/boot", ui.devices[0].Name)

	ui.setSorting("name") // sort by name desc
	assert.Equal(t, "/dev/root", ui.devices[0].Name)
}

func TestSortDevicesByUsedSize(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, true, false, false)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)

	ui.setSorting("size") // sort by used size asc
	assert.Equal(t, "/dev/boot", ui.devices[0].Name)

	ui.setSorting("size") // sort by used size desc
	assert.Equal(t, "/dev/root", ui.devices[0].Name)
}

func getAnalyzedPathWithSorting(sortBy string, sortOrder string, apparentSize bool) *UI {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, apparentSize, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.sortBy = sortBy
	ui.sortOrder = sortOrder
	if err := ui.AnalyzePath("test_dir", nil); err != nil {
		panic(err)
	}

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	return ui
}
