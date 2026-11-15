package report

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelWarn)
}

func TestMain(m *testing.M) {
	os.Unsetenv("BLOCK_SIZE")
	os.Unsetenv("BLOCKSIZE")
	os.Exit(m.Run())
}

type errorExportItem struct {
	*analyze.Dir
	err error
}

func (item errorExportItem) EncodeJSON(io.Writer, bool, fs.JSONAttributes) error {
	return item.err
}

func TestAnalyzePath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestExportDirReturnsEncodeError(t *testing.T) {
	sentinel := errors.New("encode failed")
	ui := CreateExportUI(&bytes.Buffer{}, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})

	err := ui.exportDir(errorExportItem{err: sentinel})

	assert.ErrorIs(t, err, sentinel)
}

func TestAnalyzePathWithTop(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Top: 2}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"file"`)
	assert.Contains(t, reportOutput.String(), `"name":"file2"`)
	assert.NotContains(t, reportOutput.String(), `"name":"nested"`)
	assert.NotContains(t, reportOutput.String(), `"name":"subnested"`)
}

func TestAnalyzePathWithTopAndTypeFilter(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Top: 10}, Scan: config.Scan{IncludeTypes: []string{"none"}}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.NotContains(t, reportOutput.String(), `"name":"file"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file2"`)
}

// createTestArchive writes archive.tar with a single file inside.txt into dir.
func createTestArchive(t *testing.T, dir string) {
	t.Helper()

	archive, err := os.Create(filepath.Join(dir, "archive.tar"))
	assert.Nil(t, err)
	tw := tar.NewWriter(archive)
	assert.Nil(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg, Name: "inside.txt", Size: 11, Mode: 0o644,
	}))
	_, err = tw.Write([]byte("hello world"))
	assert.Nil(t, err)
	assert.Nil(t, tw.Close())
	assert.Nil(t, archive.Close())
}

func TestAnalyzePathWithTopAndArchiveBrowsing(t *testing.T) {
	dir := t.TempDir()
	createTestArchive(t, dir)

	var output, reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Top: 5}, Scan: config.Scan{ArchiveBrowsing: true}})

	// Files inside an archive are not *analyze.File, so collecting the top
	// items used to panic instead of exporting them.
	var err error
	assert.NotPanics(t, func() {
		err = ui.AnalyzePath(dir, nil)
	})
	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"inside.txt"`)
}

func TestAnalyzePathWithDepthAndArchiveBrowsing(t *testing.T) {
	dir := t.TempDir()
	createTestArchive(t, dir)

	var output, reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Depth: 2}, Scan: config.Scan{ArchiveBrowsing: true}})

	var err error
	assert.NotPanics(t, func() {
		err = ui.AnalyzePath(dir, nil)
	})
	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"archive.tar"`)
	assert.Contains(t, reportOutput.String(), `"name":"inside.txt"`)
}

func TestAnalyzePathWithDepthCutsInsideArchive(t *testing.T) {
	dir := t.TempDir()
	createTestArchive(t, dir)

	var output, reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Depth: 1}, Scan: config.Scan{ArchiveBrowsing: true}})
	assert.Nil(t, ui.AnalyzePath(dir, nil))

	assert.Contains(t, reportOutput.String(), `"name":"archive.tar"`)
	assert.NotContains(t, reportOutput.String(), `"name":"inside.txt"`)
}

func TestAnalyzePathWithDepth(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Depth: 2}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
	assert.Contains(t, reportOutput.String(), `"name":"file2"`)
	assert.Contains(t, reportOutput.String(), `"name":"subnested"`)
}

func TestAnalyzePathWithDepthOne(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Depth: 1}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file2"`)
	assert.NotContains(t, reportOutput.String(), `"name":"subnested"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file"`)

	readDir, err := analyze.ReadAnalysis(bytes.NewReader(reportOutput.Bytes()))
	assert.Nil(t, err)
	assert.Equal(t, "test_dir", readDir.GetName())

	var nested fs.Item
	for f := range readDir.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "nested" {
			nested = f
		}
	}
	assert.NotNil(t, nested)
	assert.True(t, nested.IsDir())
}

func TestAnalyzePathWithSummarize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Summarize: true}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"test_dir"`)
	assert.NotContains(t, reportOutput.String(), `"name":"nested"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file"`)
}

func TestAnalyzePathWithTopRoundTrip(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Top: 2}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()
	assert.Nil(t, err)

	result := reportOutput.Bytes()
	assert.Contains(t, string(result), `"name":"file"`)
	assert.Contains(t, string(result), `"name":"file2"`)

	readDir, err := analyze.ReadAnalysis(bytes.NewReader(result))
	assert.Nil(t, err)
	assert.Equal(t, "test_dir", readDir.GetName())

	var names []string
	for f := range readDir.GetFiles(fs.SortByName, fs.SortAsc) {
		names = append(names, f.GetName())
	}
	assert.Equal(t, []string{"file", "file2"}, names)
}

func TestAnalyzePathWithTopLargerThanFileCount(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Top: 100}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"file"`)
	assert.Contains(t, reportOutput.String(), `"name":"file2"`)
}

func TestAnalyzePathWithDepthLargerThanTreeDepth(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Depth: 100}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
	assert.Contains(t, reportOutput.String(), `"name":"subnested"`)
	assert.Contains(t, reportOutput.String(), `"name":"file"`)
}

func TestAnalyzePathWithSummarizeAndTop(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Top: 2, Summarize: true}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"test_dir"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file"`)
	assert.NotContains(t, reportOutput.String(), `"name":"nested"`)
}

func TestAnalyzePathWithSummarizeAndDepth(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Depth: 1, Summarize: true}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"test_dir"`)
	assert.NotContains(t, reportOutput.String(), `"name":"nested"`)
	assert.NotContains(t, reportOutput.String(), `"name":"file"`)
}

// The depth limit applies to directories; a file is exported as it is.
func TestExportWithDepthKeepsAFile(t *testing.T) {
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Depth: 1}})
	err := ui.exportDir(&analyze.File{Name: "file", Size: 5})

	assert.NoError(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"file","asize":5`)
}

func TestAnalyzePathWithDepthZeroIsIgnored(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
	assert.Contains(t, reportOutput.String(), `"name":"subnested"`)
	assert.Contains(t, reportOutput.String(), `"name":"file"`)
}

func TestAnalyzePathWithProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, Progress: true, SIPrefix: true}})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestExportProgressClearsLineWithoutPadding(t *testing.T) {
	var output bytes.Buffer
	ui := CreateExportUI(&output, io.Discard, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})

	ui.scanObserver()(session.Progress{Progress: analyze.Progress{ItemCount: 1234}})
	assert.Contains(t, output.String(), "Scanning... Total items: 1,234")
	ui.scanObserver()(session.Progress{Phase: session.Finalizing})
	assert.Contains(t, output.String(), "Calculating disk usage...")

	stop := ui.showWritingProgress()
	stop()

	assert.Contains(t, output.String(), "Writing output file...")
	assert.True(t, strings.HasSuffix(output.String(), "\x1b[2K"), "the line is cleared at the end")
	assert.NotContains(t, output.String(), strings.Repeat(" ", 100))
}

// A failed export stops the spinner too.
func TestExportProgressStopsWhenWritingFails(t *testing.T) {
	var output bytes.Buffer
	ui := CreateExportUI(&output, io.Discard, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})

	err := ui.exportDir(errorExportItem{err: errors.New("encode failed")})

	assert.Error(t, err)
	assert.True(t, strings.HasSuffix(output.String(), "\x1b[2K"), "the line is cleared at the end")
}

func TestShowDevices(t *testing.T) {
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	err := ui.ListDevices(device.Getter)

	assert.Contains(t, err.Error(), "not supported")
}

func TestReadAnalysisWhileExporting(t *testing.T) {
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	err := ui.ReadAnalysis(&output)

	assert.Contains(t, err.Error(), "not possible while exporting")
}

func TestExportToFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	reportOutput, err := os.OpenFile("output.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	assert.Nil(t, err)
	defer func() {
		os.Remove("output.json")
	}()

	var output bytes.Buffer

	ui := CreateExportUI(&output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	err = ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()
	assert.Nil(t, err)

	reportOutput, err = os.OpenFile("output.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)
	_, err = reportOutput.Seek(0, 0)
	assert.Nil(t, err)
	buff := make([]byte, 200)
	_, err = reportOutput.Read(buff)
	assert.Nil(t, err)

	assert.Contains(t, string(buff), `"name":"nested"`)
}

func TestExportToFileWithTop(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	reportOutput, err := os.OpenFile("output.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	assert.Nil(t, err)

	var output bytes.Buffer

	ui := CreateExportUI(&output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true, Top: 2}})
	err = ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()
	assert.Nil(t, err)

	buff, err := os.ReadFile("output.json")
	assert.Nil(t, err)

	assert.Contains(t, string(buff), `"name":"file"`)
	assert.Contains(t, string(buff), `"name":"file2"`)
	assert.NotContains(t, string(buff), `"name":"nested"`)
}

func TestExportToFileWithDepth(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	reportOutput, err := os.OpenFile("output.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	assert.Nil(t, err)

	var output bytes.Buffer

	ui := CreateExportUI(&output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true, Depth: 2}})
	err = ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()
	assert.Nil(t, err)

	reportOutput, err = os.OpenFile("output.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)
	_, err = reportOutput.Seek(0, 0)
	assert.Nil(t, err)
	buff := make([]byte, 400)
	_, err = reportOutput.Read(buff)
	assert.Nil(t, err)

	assert.Contains(t, string(buff), `"name":"nested"`)
	assert.Contains(t, string(buff), `"name":"file2"`)
	assert.Contains(t, string(buff), `"name":"subnested"`)

	_, err = reportOutput.Seek(0, 0)
	assert.Nil(t, err)
	readDir, err := analyze.ReadAnalysis(reportOutput)
	assert.Nil(t, err)
	assert.Equal(t, "test_dir", readDir.GetName())
}

func TestFormatSize(t *testing.T) {
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "KiB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MiB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GiB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TiB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PiB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EiB")
	assert.Contains(t, ui.formatSize(-1<<10-1), "KiB")
}

func TestFormatSizeWithBlockSizeEnvironment(t *testing.T) {
	t.Setenv("BLOCK_SIZE", "1K")
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	assert.Equal(t, "2", ui.formatSize(1025))
}

func TestFormatSizeDec(t *testing.T) {
	var output bytes.Buffer
	var reportOutput bytes.Buffer

	ui := CreateExportUI(&output, &reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true, SIPrefix: true}})

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "kB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EB")
	assert.Contains(t, ui.formatSize(-1<<10-1), "kB")
}

func TestAnalyzePathsRejectsMultipleDirs(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 10))
	ui := CreateExportUI(buff, buff, analyze.CreateAnalyzer(), &config.Settings{})

	err := ui.AnalyzePaths([]string{"test_dir", "other_dir"})

	assert.ErrorContains(t, err, "more than one directory is not supported")
}
