package app

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/testdev"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
)

// Flags which contradict each other are refused before anything is set up,
// rather than one of them being silently ignored.
func TestConflictingFlagsAreRefused(t *testing.T) {
	for _, tc := range []struct {
		flags Flags
		want  string
	}{
		{Flags{NoPrefix: true, UseSIPrefix: true}, "--no-prefix and --si"},
		{Flags{Interactive: true, NonInteractive: true}, "--interactive and --non-interactive"},
		{Flags{SequentialScanning: true, DbPath: "x.badger"}, "--sequential and --db"},
		{Flags{Web: true, OutputFile: "out.json"}, "--web and --output-file"},
		{Flags{Web: true, ShowDisks: true}, "--web and --show-disks"},
		{Flags{InputFile: "in.json", ReadFromStorage: true, DbPath: "x.badger"}, "--input-file and --read-from-storage"},
		{Flags{ShowDisks: true, InputFile: "in.json"}, "--show-disks and --input-file"},
		{Flags{ShowDisks: true, ReadFromStorage: true, DbPath: "x.badger"}, "--show-disks and --read-from-storage"},
		{Flags{OutputFile: "out.json", ShowDisks: true}, "--output-file and --show-disks"},
		{Flags{OutputFile: "out.json", InputFile: "in.json"}, "--output-file and --input-file"},
	} {
		tc.flags.LogFile = "/dev/null"
		_, err := runApp(&tc.flags, []string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})

		assert.EqualError(t, err, tc.want+" cannot be used at once")
	}
}

// Resolving the settings leaves the flags as they were: they are the
// configuration the user gave, which --write-config saves.
func TestSettingsLeaveTheFlagsAlone(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	mount := filepath.Join(root, "mnt")
	flags := &Flags{NoCross: true, IgnoreDirs: []string{"/x"}}
	a := &App{Flags: flags, Getter: mountsGetter{mount}}

	s, err := a.settings([]string{root})

	require.NoError(t, err)
	assert.Equal(t, []string{"/x", mount}, s.Scan.IgnoreDirs)
	assert.Equal(t, []string{"/x"}, flags.IgnoreDirs)
}

// The terminal and web UIs use colours on their own screen; stdout and the
// export only when writing to a terminal.
func TestSettingsDisplayFollowsTheFrontend(t *testing.T) {
	for _, tc := range []struct {
		flags          Flags
		istty          bool
		colors, progrs bool
	}{
		{Flags{}, true, true, false},
		{Flags{NoColor: true}, true, false, false},
		{Flags{Web: true}, false, true, false},
		{Flags{NonInteractive: true}, true, true, true},
		{Flags{NonInteractive: true}, false, false, false},
		{Flags{NonInteractive: true, NoProgress: true, NoColor: true}, true, false, false},
		{Flags{OutputFile: "out.json"}, true, true, true},
		{Flags{OutputFile: "out.json"}, false, false, false},
	} {
		a := &App{Flags: &tc.flags, Istty: tc.istty}

		s, err := a.settings(nil)

		require.NoError(t, err)
		assert.Equal(t, tc.colors, s.Display.Colors, "%+v tty=%v", tc.flags, tc.istty)
		assert.Equal(t, tc.progrs, s.Display.Progress, "%+v tty=%v", tc.flags, tc.istty)
	}
}

// The analyzer is chosen in one place.
func TestAnalyzerChoice(t *testing.T) {
	for _, tc := range []struct {
		flags Flags
		want  any
	}{
		{Flags{}, &analyze.ParallelAnalyzer{}},
		{Flags{SequentialScanning: true}, &analyze.SequentialAnalyzer{}},
		{Flags{NonInteractive: true}, &analyze.TopDirAnalyzer{}},
		{Flags{NonInteractive: true, Top: 3}, &analyze.ParallelAnalyzer{}},
		{Flags{NonInteractive: true, Depth: 1}, &analyze.ParallelAnalyzer{}},
		{Flags{NonInteractive: true, SequentialScanning: true}, &analyze.SequentialAnalyzer{}},
		{Flags{DbPath: filepath.Join(t.TempDir(), "a.badger")}, &analyze.StoredAnalyzer{}},
		{Flags{NonInteractive: true, DbPath: filepath.Join(t.TempDir(), "a.sqlite")}, &analyze.SqliteAnalyzer{}},
	} {
		a := &App{Flags: &tc.flags, Istty: true}
		s, err := a.settings(nil)
		require.NoError(t, err)

		analyzer, err := a.analyzer(a.frontend(), &s)

		require.NoError(t, err)
		assert.IsType(t, tc.want, analyzer, "%+v", tc.flags)
	}
}

// mountsGetter reports one mount point.
type mountsGetter struct{ mount string }

func (m mountsGetter) GetDevicesInfo() (device.Devices, error) { return nil, nil }

func (m mountsGetter) GetMounts() (device.Devices, error) {
	return device.Devices{{Name: "/dev/x", MountPoint: m.mount}}, nil
}
