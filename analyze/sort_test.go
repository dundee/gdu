package analyze

import (
	"sort"
	"testing"

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

	assert.Equal(t, int64(3), files[0].Usage)
	assert.Equal(t, int64(2), files[1].Usage)
	assert.Equal(t, int64(1), files[2].Usage)
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

	assert.Equal(t, int64(1), files[0].Size)
	assert.Equal(t, int64(2), files[1].Size)
	assert.Equal(t, int64(3), files[2].Size)
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

	assert.Equal(t, int64(3), files[0].Size)
	assert.Equal(t, int64(2), files[1].Size)
	assert.Equal(t, int64(1), files[2].Size)
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

	assert.Equal(t, int64(1), files[0].Size)
	assert.Equal(t, int64(2), files[1].Size)
	assert.Equal(t, int64(3), files[2].Size)
}

func TestSortByItemCount(t *testing.T) {
	files := Files{
		&File{
			ItemCount: 1,
		},
		&File{
			ItemCount: 2,
		},
		&File{
			ItemCount: 3,
		},
	}

	sort.Sort(ByItemCount(files))

	assert.Equal(t, 3, files[0].ItemCount)
	assert.Equal(t, 2, files[1].ItemCount)
	assert.Equal(t, 1, files[2].ItemCount)
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

	assert.Equal(t, "cc", files[0].Name)
	assert.Equal(t, "bb", files[1].Name)
	assert.Equal(t, "aa", files[2].Name)
}
