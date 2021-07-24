package tui

import (
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeByApparentSize(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "desc", true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestSortByApparentSizeAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "asc", true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "aaa")
}

func TestAnalyzeBySize(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "desc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestSortBySizeAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("size", "asc", false)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ddd")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "aaa")
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
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
	assert.Contains(t, ui.table.GetCell(2, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(3, 0).Text, "ddd")
}

func TestAnalyzeByItemCountAsc(t *testing.T) {
	ui := getAnalyzedPathWithSorting("itemCount", "asc", false)

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

func getAnalyzedPathWithSorting(sortBy string, sortOrder string, apparentSize bool) *UI {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, apparentSize)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.sortBy = sortBy
	ui.sortOrder = sortOrder
	if err := ui.AnalyzePath("test_dir", nil); err != nil {
		panic(err)
	}

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	return ui
}
