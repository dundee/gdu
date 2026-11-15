package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/testdir"
)

// ncduTotals adds up an export the way ncdu reads it: every item, directories
// included, counts its own asize and dsize, which default to 0.
func ncduTotals(t *testing.T, node any) (asize, dsize int64) {
	t.Helper()
	switch node := node.(type) {
	case []any:
		for _, child := range node {
			a, d := ncduTotals(t, child)
			asize, dsize = asize+a, dsize+d
		}
	case map[string]any:
		if v, ok := node["asize"].(float64); ok {
			asize = int64(v)
		}
		if v, ok := node["dsize"].(float64); ok {
			dsize = int64(v)
		}
	default:
		t.Fatalf("unexpected JSON value %T", node)
	}
	return asize, dsize
}

// exportOf scans test_dir with the analyzer a, as gdu does, and returns the
// tree and its export with the header gdu writes.
func exportOf(t *testing.T, a Analyzer) (Node, []byte) {
	t.Helper()
	root := a.AnalyzeDir(context.Background(), "test_dir", Options{})
	root.UpdateStats(false)
	var out bytes.Buffer
	out.WriteString(`[1,2,{"progname":"gdu","progver":"test","timestamp":0},` + "\n")
	require.NoError(t, root.EncodeJSON(&out, true, nil))
	out.WriteString("]")
	return root, out.Bytes()
}

func exportingAnalyzers(t *testing.T) map[string]Analyzer {
	sqlite, err := createSqliteAnalyzer(filepath.Join(t.TempDir(), "gdu.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { sqlite.storage.Close() })
	return map[string]Analyzer{
		"memory": CreateAnalyzer(),
		"badger": createStoredAnalyzer(filepath.Join(t.TempDir(), "gdu.badger")),
		"sqlite": sqlite,
	}
}

// ncdu adds up the sizes of every item of an export; directories gave their
// totals, which ncdu counted again at every level above them.
func TestExportAddsUpAsNcduReadsIt(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	for name, a := range exportingAnalyzers(t) {
		t.Run(name, func(t *testing.T) {
			root, data := exportOf(t, a)
			var doc []any
			require.NoError(t, json.Unmarshal(data, &doc))

			asize, dsize := ncduTotals(t, doc[3])

			assert.Equal(t, root.GetSize(), asize)
			assert.Equal(t, root.GetUsage(), dsize)
		})
	}
}

// gdu reads its export back with the totals it wrote, also of directories it
// exported without their entries.
func TestExportReadsBackWithItsTotals(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	for name, a := range exportingAnalyzers(t) {
		t.Run(name, func(t *testing.T) {
			root, data := exportOf(t, a)

			read, err := ReadAnalysis(bytes.NewReader(data))
			require.NoError(t, err)
			read.UpdateStats(false)

			assert.Equal(t, root.GetSize(), read.GetSize())
			assert.Equal(t, root.GetUsage(), read.GetUsage())
			assert.Equal(t, root.GetItemCount(), read.GetItemCount())
		})
	}
}

// A depth-limited export keeps the totals of the directories it cut.
func TestTruncatedExportKeepsItsTotals(t *testing.T) {
	input := `[1,2,{"progname":"gdu"},
		[{"name":"/root","dsize":4096,"gdu_asize":300,"gdu_dsize":16384,"items":4},
		[{"name":"cut","dsize":4096,"gdu_asize":300,"gdu_dsize":12288,"items":3}]]]`

	dir, err := ReadAnalysis(bytes.NewBufferString(input))
	require.NoError(t, err)
	dir.UpdateStats(false)

	assert.Equal(t, int64(300), dir.GetSize())
	assert.Equal(t, int64(16384), dir.GetUsage())
	assert.Equal(t, int64(4), dir.GetItemCount())
	assert.Equal(t, int64(4096), dir.OwnUsage)
}

// In an export by ncdu, a directory's asize and dsize are its own.
func TestReadNcduExport(t *testing.T) {
	input := `[1,2,{"progname":"ncdu","progver":"2.4","timestamp":0},
		[{"name":"/root","asize":4096,"dsize":8192},
		{"name":"f","asize":100,"dsize":4096},
		[{"name":"sub","asize":4096,"dsize":4096},
		{"name":"g","asize":200,"dsize":4096}]]]`

	dir, err := ReadAnalysis(bytes.NewBufferString(input))
	require.NoError(t, err)
	dir.UpdateStats(false)

	assert.Equal(t, int64(8192+4096+4096+4096), dir.GetUsage(), "as ncdu shows it")
	assert.Equal(t, int64(300), dir.GetSize())
	assert.Equal(t, int64(4096+4096), dir.Files[1].GetUsage())
}
