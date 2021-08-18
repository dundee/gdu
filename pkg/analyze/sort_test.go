package analyze

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSortByUsage(t *testing.T) {
	files := Files{
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

	sort.Sort(files)

	assert.Equal(t, int64(3), files[0].GetUsage())
	assert.Equal(t, int64(2), files[1].GetUsage())
	assert.Equal(t, int64(1), files[2].GetUsage())
}

func TestSortByUsageAsc(t *testing.T) {
	files := Files{
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

	sort.Sort(sort.Reverse(files))

	assert.Equal(t, int64(1), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(3), files[2].GetSize())
}

func TestSortBySize(t *testing.T) {
	files := Files{
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

	sort.Sort(ByApparentSize(files))

	assert.Equal(t, int64(3), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(1), files[2].GetSize())
}

func TestSortBySizeAsc(t *testing.T) {
	files := Files{
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

	sort.Sort(sort.Reverse(ByApparentSize(files)))

	assert.Equal(t, int64(1), files[0].GetSize())
	assert.Equal(t, int64(2), files[1].GetSize())
	assert.Equal(t, int64(3), files[2].GetSize())
}

func TestSortByItemCount(t *testing.T) {
	files := Files{
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

	sort.Sort(ByItemCount(files))

	assert.Equal(t, 3, files[0].GetItemCount())
	assert.Equal(t, 2, files[1].GetItemCount())
	assert.Equal(t, 1, files[2].GetItemCount())
}

func TestSortByName(t *testing.T) {
	files := Files{
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

	sort.Sort(ByName(files))

	assert.Equal(t, "cc", files[0].GetName())
	assert.Equal(t, "bb", files[1].GetName())
	assert.Equal(t, "aa", files[2].GetName())
}

func TestSortByMtime(t *testing.T) {
	files := Files{
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

	sort.Sort(ByMtime(files))

	assert.Equal(t, 42, files[0].GetMtime().Minute())
	assert.Equal(t, 41, files[1].GetMtime().Minute())
	assert.Equal(t, 40, files[2].GetMtime().Minute())
}
