package app

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/device"
	gfs "github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestVersion(t *testing.T) {
	out, err := runApp(
		&Flags{ShowVersion: true},
		[]string{},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "Version:\t development")
	assert.Nil(t, err)
}

func TestShouldRunInNonInteractiveModeInteractiveOverridesNoTTY(t *testing.T) {
	flags := &Flags{Interactive: true}

	assert.False(t, flags.ShouldRunInNonInteractiveMode(false))
}

func TestShouldRunInNonInteractiveMode(t *testing.T) {
	flags := &Flags{NonInteractive: true}

	assert.True(t, flags.ShouldRunInNonInteractiveMode(false))
}

func TestShouldRunInNonInteractiveModeInteractiveKeepsNonInteractiveOnlyFlags(t *testing.T) {
	flags := &Flags{Interactive: true, Summarize: true}

	assert.True(t, flags.ShouldRunInNonInteractiveMode(false))
}

func TestInteractiveAndNonInteractiveConflict(t *testing.T) {
	out, err := runApp(
		&Flags{Interactive: true, NonInteractive: true},
		[]string{"."},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.ErrorContains(t, err, "--interactive and --non-interactive cannot be used at once")
}

func TestAnalyzePath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithShowItemCountNonInteractive(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowItemCount: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.Regexp(t, regexp.MustCompile(`(?m)\s+\d+\s+/nested$`), out)
}

func TestSequentialScanning(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", SequentialScanning: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestFollowSymlinks(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", FollowSymlinks: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestShowAnnexedSize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowAnnexedSize: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathProfiling(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", Profiling: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithIgnoring(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:           "/dev/null",
			IgnoreDirPatterns: []string{"/(abc)+"},
			NoHidden:          true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithIgnoringPatternError(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:           "/dev/null",
			IgnoreDirPatterns: []string{"[[["},
			NoHidden:          true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Equal(t, out, "")
	assert.NotNil(t, err)
}

func TestAnalyzePathWithIgnoringFromNotExistingFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:        "/dev/null",
			IgnoreFromFile: "file",
			NoHidden:       true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Equal(t, out, "")
	assert.NotNil(t, err)
}

func TestAnalyzePathWithGui(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null"},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathWithGuiNoColor(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoColor: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiShowMTimeAndItemCount(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowItemCount: true, ShowMTime: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiNoDelete(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoDelete: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiNoViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoViewFile: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiNoSpawnShell(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoSpawnShell: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiDeleteInParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DeleteInParallel: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathWithGuiBackgroundDeletion(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DeleteInBackground: true},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathWithDefaultSorting(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile: "/dev/null",
			Sorting: Sorting{
				By:    "name",
				Order: "asc",
			},
		},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathWithStyle(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile: "/dev/null",
			Style: Style{
				SelectedRow: ColorStyle{
					TextColor:       "black",
					BackgroundColor: "red",
				},
				Marked: ColorStyle{
					TextColor:       "white",
					BackgroundColor: "blue",
				},
				ProgressModal: ProgressModalOpts{
					CurrentItemNameMaxLen: 10,
				},
				Footer: FooterColorStyle{
					TextColor:       "black",
					BackgroundColor: "red",
					NumberColor:     "white",
				},
				Header: HeaderColorStyle{
					TextColor:       "black",
					BackgroundColor: "red",
					Hidden:          true,
				},
				ResultRow: ResultRowColorStyle{
					NumberColor:    "orange",
					DirectoryColor: "blue",
				},
				UseOldSizeBar: true,
			},
		},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathNoUnicode(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:   "/dev/null",
			NoUnicode: true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithExport(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "output.json"},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NotEmpty(t, out)
	assert.Nil(t, err)
}

func TestAnalyzePathWithChdir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:   "/dev/null",
			ChangeCwd: true,
		},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestReadAnalysisFromFile(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", InputFile: "../../../internal/testdata/test.json"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NotEmpty(t, out)
	assert.Contains(t, out, "main.go")
	assert.Nil(t, err)
}

func TestReadWrongAnalysisFromFile(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", InputFile: "../../../internal/testdata/wrong.json"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "array of maps not found")
}

func TestWrongCombinationOfPrefixes(t *testing.T) {
	out, err := runApp(
		&Flags{NoPrefix: true, UseSIPrefix: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "cannot be used at once")
}

func TestReadWrongAnalysisFromNotExistingFile(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", InputFile: "xxx.json"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestAnalyzePathWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBufferString("")

	app := App{
		Flags:       &Flags{LogFile: "/dev/null"},
		Args:        []string{"xxx"},
		Istty:       false,
		Writer:      buff,
		TermApp:     testapp.CreateMockedApp(false),
		Getter:      testdev.DevicesInfoGetterMock{},
		PathChecker: os.Stat,
	}
	err := app.Run()

	assert.Equal(t, "", strings.TrimSpace(buff.String()))
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestNoCross(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoCross: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestListDevices(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowDisks: true},
		[]string{},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "Device")
	assert.Nil(t, err)
}

func TestListDevicesToFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowDisks: true, OutputFile: "output.json"},
		[]string{},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Equal(t, "", out)
	assert.Contains(t, err.Error(), "not supported")
}

func TestListDevicesWithGui(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ShowDisks: true},
		[]string{},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.Empty(t, out)
}

func TestMaxCores(t *testing.T) {
	_, err := runApp(
		&Flags{LogFile: "/dev/null", MaxCores: 1},
		[]string{},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Equal(t, 1, runtime.GOMAXPROCS(0))
	assert.Nil(t, err)
}

func TestMaxCoresHighEdge(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("Skipping on a single core CPU")
	}
	out, err := runApp(
		&Flags{LogFile: "/dev/null", MaxCores: runtime.NumCPU() + 1},
		[]string{},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NotEqual(t, runtime.NumCPU(), runtime.GOMAXPROCS(0))
	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestMaxCoresLowEdge(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("Skipping on a single core CPU")
	}
	out, err := runApp(
		&Flags{LogFile: "/dev/null", MaxCores: -100},
		[]string{},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NotEqual(t, runtime.NumCPU(), runtime.GOMAXPROCS(0))
	assert.Empty(t, out)
	assert.Nil(t, err)
}

type uiTimeFilterMock struct {
	timeFilter common.TimeFilter
}

func (m *uiTimeFilterMock) ListDevices(getter device.DevicesInfoGetter) error { return nil }
func (m *uiTimeFilterMock) AnalyzePath(path string, parentDir gfs.Item) error { return nil }
func (m *uiTimeFilterMock) ReadAnalysis(input io.Reader) error                { return nil }
func (m *uiTimeFilterMock) ReadFromStorage(storagePath, path string) error    { return nil }
func (m *uiTimeFilterMock) SetIgnoreTypes(types []string)                     {}
func (m *uiTimeFilterMock) SetIgnoreDirPaths(paths []string)                  {}
func (m *uiTimeFilterMock) SetIgnoreDirPatterns(paths []string) error         { return nil }
func (m *uiTimeFilterMock) SetIgnoreFromFile(ignoreFile string) error         { return nil }
func (m *uiTimeFilterMock) SetIgnoreHidden(value bool)                        {}
func (m *uiTimeFilterMock) SetIncludeTypes(types []string)                    {}
func (m *uiTimeFilterMock) SetFollowSymlinks(value bool)                      {}
func (m *uiTimeFilterMock) SetShowAnnexedSize(value bool)                     {}
func (m *uiTimeFilterMock) SetAnalyzer(analyzer common.Analyzer)              {}
func (m *uiTimeFilterMock) SetTimeFilter(timeFilter common.TimeFilter) {
	m.timeFilter = timeFilter
}
func (m *uiTimeFilterMock) SetArchiveBrowsing(value bool) {}
func (m *uiTimeFilterMock) SetCollapsePath(value bool)    {}
func (m *uiTimeFilterMock) StartUILoop() error            { return nil }

func TestSetTimeFiltersInvalid(t *testing.T) {
	a := &App{Flags: &Flags{Since: "not-a-date"}}
	ui := &uiTimeFilterMock{}

	err := a.setTimeFilters(ui)

	assert.ErrorContains(t, err, "invalid time filter")
}

func TestSetTimeFiltersSetsFilter(t *testing.T) {
	futureDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	a := &App{Flags: &Flags{Since: futureDate}}
	ui := &uiTimeFilterMock{}

	err := a.setTimeFilters(ui)

	assert.Nil(t, err)
	if assert.NotNil(t, ui.timeFilter) {
		assert.False(t, ui.timeFilter(time.Now()))
		assert.True(t, ui.timeFilter(time.Now().Add(72*time.Hour)))
	}
}

// nolint: unparam // Why: it's used in linux tests
func runApp(flags *Flags, args []string, istty bool, getter device.DevicesInfoGetter) (output string, err error) {
	buff := bytes.NewBufferString("")

	app := App{
		Flags:       flags,
		Args:        args,
		Istty:       istty,
		Writer:      buff,
		TermApp:     testapp.CreateMockedApp(false),
		Getter:      getter,
		PathChecker: testdir.MockedPathChecker,
	}
	err = app.Run()

	return strings.TrimSpace(buff.String()), err
}
