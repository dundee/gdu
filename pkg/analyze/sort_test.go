package analyze

import (
	"sort"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestSortByUsage(t *testing.T) {
	files := fs.Files{
		&File{
			Usage: 1,
		},
		&File{
			Usage: 2,
		},
		&File{
			Usage: 3,
		},
	}

	sort.Sort(sort.Reverse(files))

	assert.Equal(t, int64(3), files[0].GetUsage())
	assert.Equal(t, int64(2), files[1].GetUsage())
	assert.Equal(t, int64(1), files[2].GetUsage())
}

func TestStableSortByUsage(t *testing.T) {
	files := fs.Files{
		&File{
			Name:  "aaa",
			Usage: 1,
		},
		&File{
			Name:  "bbb",
			Usage: 1,
		},
		&File{
			Name:  "ccc",
			Usage: 3,
		},
	}

	sort.Sort(sort.Reverse(files))

	assert.Equal(t, "ccc", files[0].GetName())
	assert.Equal(t, "bbb", files[1].GetName())
	assert.Equal(t, "aaa", files[2].GetName())
}

func TestSortByUsageAsc(t *testing.T) {
	files := fs.Files{
		&File{
			Size: 1,
		},
		&File{
			Size: 2,
		},
		&File{
			Size: 3,
		},
	}

	sort.Sort(files)

	assert.Equal(t, int64(1), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(3), files[2].GetSize())
}

func TestSortBySize(t *testing.T) {
	files := fs.Files{
		&File{
			Size: 1,
		},
		&File{
			Size: 2,
		},
		&File{
			Size: 3,
		},
	}

	sort.Sort(sort.Reverse(fs.ByApparentSize(files)))

	assert.Equal(t, int64(3), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(1), files[2].GetSize())
}

func TestSortBySizeAsc(t *testing.T) {
	files := fs.Files{
		&File{
			Size: 1,
		},
		&File{
			Size: 2,
		},
		&File{
			Size: 3,
		},
	}

	sort.Sort(fs.ByApparentSize(files))

	assert.Equal(t, int64(1), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(3), files[2].GetSize())
}

func TestSortByItemCount(t *testing.T) {
	files := fs.Files{
		&Dir{
			ItemCount: 1,
		},
		&Dir{
			ItemCount: 2,
		},
		&Dir{
			ItemCount: 3,
		},
	}

	sort.Sort(sort.Reverse(fs.ByItemCount(files)))

	assert.Equal(t, 3, files[0].GetItemCount())
	assert.Equal(t, 2, files[1].GetItemCount())
	assert.Equal(t, 1, files[2].GetItemCount())
}

func TestSortByName(t *testing.T) {
	files := fs.Files{
		&File{
			Name: "aa",
		},
		&File{
			Name: "bb",
		},
		&File{
			Name: "cc",
		},
	}

	sort.Sort(sort.Reverse(fs.ByName(files)))

	assert.Equal(t, "cc", files[0].GetName())
	assert.Equal(t, "bb", files[1].GetName())
	assert.Equal(t, "aa", files[2].GetName())
}

func TestNaturalSortByNameAsc(t *testing.T) {
	files := fs.Files{
		&File{
			Name: "aa3",
		},
		&File{
			Name: "aa20",
		},
		&File{
			Name: "aa100",
		},
	}

	sort.Sort(fs.ByName(files))

	assert.Equal(t, "aa3", files[0].GetName())
	assert.Equal(t, "aa20", files[1].GetName())
	assert.Equal(t, "aa100", files[2].GetName())
}

func TestSortByMtime(t *testing.T) {
	files := fs.Files{
		&File{
			Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		},
		&File{
			Mtime: time.Date(2021, 8, 19, 0, 41, 0, 0, time.UTC),
		},
		&File{
			Mtime: time.Date(2021, 8, 19, 0, 42, 0, 0, time.UTC),
		},
	}

	sort.Sort(sort.Reverse(fs.ByMtime(files)))

	assert.Equal(t, 42, files[0].GetMtime().Minute())
	assert.Equal(t, 41, files[1].GetMtime().Minute())
	assert.Equal(t, 40, files[2].GetMtime().Minute())
}
