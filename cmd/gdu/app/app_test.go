package app

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/device"
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
			IgnoreDirPatterns: []string{"/[abc]+"},
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
				ProgressModal: ProgressModalOpts{
					CurrentItemNameMaxLen: 10,
				},
			},
		},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
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
	assert.Contains(t, err.Error(), "Array of maps not found")
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

func runApp(flags *Flags, args []string, istty bool, getter device.DevicesInfoGetter) (string, error) {
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
	err := app.Run()

	return strings.TrimSpace(buff.String()), err
}
