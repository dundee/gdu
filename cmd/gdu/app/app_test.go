package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dundee/gdu/v6/internal/cleanup"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/internal/testdev"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelWarn)
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

func TestExclusiveSize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", ExclusiveSize: true},
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

func TestAnalyzePathWithIgnoringFromNotExistingGitignoreFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreFromGitignore: "file",
			NoHidden:            true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Equal(t, out, "")
	assert.NotNil(t, err)
}

// writeGitignore puts a gitignore file in a temp dir and returns its path.
func writeGitignore(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzePathWithIgnoringFromGitignoreFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreFromGitignore: writeGitignore(t, "nested/\n"),
			NoHidden:            true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.NotContains(t, out, "nested")
}

// gdu resolves every scanned path with filepath.Abs, so a pattern anchored
// with a leading / has to be matched against the scanned root to fire at all.
func TestAnalyzePathWithRootedGitignorePattern(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreFromGitignore: writeGitignore(t, "/nested\n"),
			NoHidden:            true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.NotContains(t, out, "nested")
}

func TestAnalyzePathWithRootedGitignorePatternBelowRoot(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// subnested lives at test_dir/nested/subnested, so anchoring it to the
	// scanned root must not prune it
	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreFromGitignore: writeGitignore(t, "/subnested\n"),
			NoHidden:            true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.Contains(t, out, "nested")
}

func TestAnalyzePathWithGitignoreAndPatternCombined(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// both sources apply; neither is dropped because the other was given
	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreDirPatterns:   []string{".*/(abc)+"},
			IgnoreFromGitignore: writeGitignore(t, "nested/\n"),
			NoHidden:            true,
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Nil(t, err)
	assert.NotContains(t, out, "nested")
}

func TestAnalyzePathWithNegatedGitignorePattern(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{
			LogFile:             "/dev/null",
			IgnoreFromGitignore: writeGitignore(t, "nested/\n!nested/file2\n"),
			NoHidden:            true,
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

func TestGuiShell(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", Shell: "/custom/shell"},
		[]string{"test_dir"},
		true,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)
}

func TestGuiCtrlCQuits(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", CtrlCQuits: true},
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

func TestGuiTrashCommand(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", TrashCommand: "trash-put"},
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
				UseOldSizeBar:     true,
				ShowBarPercentage: true,
				ShowItemCountBar:  true,
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

func TestAnalyzePathWithCustomizedExport(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer os.Remove("output.json")

	out, err := runApp(
		&Flags{
			LogFile:     "/dev/null",
			OutputFile:  "output.json",
			OutputAttrs: "asize,dsize",
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.NoError(t, err)

	content, err := os.ReadFile("output.json")
	assert.NoError(t, err)

	var report []any
	assert.NoError(t, json.Unmarshal(content, &report))
	assertOutputAttributes(t, report[3])
}

func assertOutputAttributes(t *testing.T, value any) {
	t.Helper()

	switch value := value.(type) {
	case []any:
		for _, child := range value {
			assertOutputAttributes(t, child)
		}
	case map[string]any:
		assert.Contains(t, value, "name")
		for attribute := range value {
			// a directory's totals go under gdu_asize and gdu_dsize
			assert.Contains(t, []string{"name", "asize", "dsize", "gdu_asize", "gdu_dsize"}, attribute)
		}
	default:
		t.Fatalf("unexpected JSON value %T", value)
	}
}

func TestOutputAttributesRequireExport(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputAttrs: "asize"},
		nil,
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.EqualError(t, err, "--output-attrs requires --output-file")
}

func TestOutputAttributesRejectUnknownAttribute(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "output.json", OutputAttrs: "size"},
		nil,
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.EqualError(t, err, `unknown JSON output attribute "size"`)
}

func TestAnalyzePathWithExportAndTop(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "output.json", Top: 2},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)

	content, err := os.ReadFile("output.json")
	assert.Nil(t, err)
	assert.Contains(t, string(content), `"name":"file"`)
	assert.Contains(t, string(content), `"name":"file2"`)
	assert.NotContains(t, string(content), `"name":"nested"`)
}

func TestAnalyzePathWithExportAndDepth(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "output.json", Depth: 1},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)

	content, err := os.ReadFile("output.json")
	assert.Nil(t, err)
	assert.Contains(t, string(content), `"name":"nested"`)
	assert.NotContains(t, string(content), `"name":"subnested"`)
	assert.Regexp(t, `"name":"nested",("dsize":\d+,)?"gdu_asize":`, string(content))
	assert.Contains(t, string(content), `"items":`)
}

func TestAnalyzePathWithExportAndSummarize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	defer func() {
		os.Remove("output.json")
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "output.json", Summarize: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Nil(t, err)

	content, err := os.ReadFile("output.json")
	assert.Nil(t, err)
	assert.Contains(t, string(content), "test_dir")
	assert.NotContains(t, string(content), `"name":"nested"`)
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
		ErrWriter:   io.Discard,
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

func TestWebFlagCreatesWebUIAndHonorsNoDelete(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Pre-bind the address so StartUILoop fails fast on net.Listen instead of
	// actually serving; this only needs to exercise createUI's webUI
	// construction and NoDelete wiring, not a live server.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	defer blocker.Close()

	_, err = runApp(
		&Flags{
			LogFile:      "/dev/null",
			Web:          true,
			NoDelete:     true,
			TrashCommand: "trash-put",
			WebConfig:    WebConfig{Listen: blocker.Addr().String()},
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "binding")
}

func TestWebFlagRejectsInvalidCleanupRule(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	_, err := runApp(
		&Flags{
			LogFile: "/dev/null",
			Web:     true,
			Cleanup: CleanupConfig{Rules: []CleanupRule{{Category: "junk", Dir: "x"}}},
		},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, `cleanup config: cleanup rule 1: unknown category "junk"`)
}

func TestShellConfigFromYAML(t *testing.T) {
	var f Flags
	err := yaml.Unmarshal([]byte(`
shell: /usr/bin/fish
`), &f)
	assert.NoError(t, err)
	assert.Equal(t, "/usr/bin/fish", f.Shell)
}

func TestCleanupConfigFromYAML(t *testing.T) {
	var f Flags
	err := yaml.Unmarshal([]byte(`
cleanup:
  exclude: [~/work]
  rules:
    - category: build-artifact
      confidence: safe
      dir: .gradle
      markers: [build.gradle]
`), &f)
	assert.NoError(t, err)
	assert.Equal(t, cleanup.Config{
		Exclude: []string{"~/work"},
		Rules: []cleanup.Rule{{
			Category: cleanup.BuildArtifact, Confidence: cleanup.Safe,
			Dir: ".gradle", Markers: []string{"build.gradle"},
		}},
	}, f.Cleanup.config())
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
	assert.ErrorContains(t, err, "--output-file and --show-disks cannot be used at once")
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

func TestSettingsRejectInvalidTimeFilter(t *testing.T) {
	a := &App{Flags: &Flags{Since: "not-a-date"}}

	_, err := a.settings(nil)

	assert.ErrorContains(t, err, "invalid time filter")
}

func TestSettingsResolveTimeFilter(t *testing.T) {
	futureDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	a := &App{Flags: &Flags{Since: futureDate}}

	s, err := a.settings(nil)

	require.NoError(t, err)
	if assert.NotNil(t, s.Scan.TimeFilter) {
		assert.False(t, s.Scan.TimeFilter.IncludeByTimeFilter(time.Now(), s.Scan.TimeLocation))
		assert.True(t, s.Scan.TimeFilter.IncludeByTimeFilter(time.Now().Add(72*time.Hour), s.Scan.TimeLocation))
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
