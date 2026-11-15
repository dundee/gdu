// Package config holds the settings of one run of gdu: the command line and
// the config files, resolved once by cmd/gdu/app, validated, and not changed
// afterwards. The frontends are built from them.
package config

import (
	"time"

	"github.com/dundee/gdu/v6/internal/cleanup"
	"github.com/dundee/gdu/v6/internal/ignore"
	"github.com/dundee/gdu/v6/internal/timefilter"
	"github.com/dundee/gdu/v6/pkg/fs"
)

// Settings are the settings of one run.
type Settings struct {
	Scan    Scan
	Display Display
	Delete  Delete
	TUI     TUI
	Export  Export
	Web     Web
}

// Scan says what a scan leaves out and how it counts what it finds.
type Scan struct {
	// IgnoreDirs are the absolute paths of the directories left out,
	// mount points below the scanned roots included with --no-cross
	IgnoreDirs []string
	// IgnoreDirPattern matches the paths of the directories left out, from
	// --ignore-dirs-pattern, --ignore-from and --ignore-from-gitignore; nil
	// matches none
	IgnoreDirPattern *ignore.Pattern
	IgnoreHidden     bool
	// IgnoreOtherFilesystems leaves out what lies on another filesystem than
	// its scanned root
	IgnoreOtherFilesystems bool
	// IncludeTypes and ExcludeTypes are file extensions, without the dot
	IncludeTypes []string
	ExcludeTypes []string
	// TimeFilter counts files by their mtime; nil counts all
	TimeFilter *timefilter.TimeFilter
	// TimeLocation is where the dates of TimeFilter are read
	TimeLocation    *time.Location
	FollowSymlinks  bool
	AnnexedSize     bool
	ExclusiveSize   bool
	ArchiveBrowsing bool
}

// Display says how sizes, counts and items are shown.
type Display struct {
	Colors   bool
	Progress bool
	// NoUnicode draws progress and bars with ASCII characters
	NoUnicode    bool
	ApparentSize bool
	RelativeSize bool
	SIPrefix     bool
	NoPrefix     bool
	// FixedUnit shows every size in this unit ("k"), or in the best one when
	// empty
	FixedUnit     string
	ItemCount     bool
	MTime         bool
	SymlinkTarget bool
	CollapsePath  bool
	// SortBy and SortOrder are the default sorting, such as "size" and
	// "desc"; empty for gdu's default
	SortBy      string
	SortOrder   string
	ReverseSort bool
	Top         int
	Depth       int
	Summarize   bool
}

// Delete says what deleting may do and how.
type Delete struct {
	Disabled     bool
	InBackground bool
	InParallel   bool
	// TrashCommand moves an item to the trash instead of the system trash
	TrashCommand string
}

// TUI configures the interactive terminal UI.
type TUI struct {
	Style            Style
	Shell            string
	NoViewFile       bool
	NoSpawnShell     bool
	NoConfirmQuit    bool
	CtrlCQuits       bool
	ChangeCwd        bool
	BrowseParentDirs bool
}

// Style holds the colours and looks of the terminal UI; an empty colour
// keeps the default.
type Style struct {
	SelectedRowText       string
	SelectedRowBackground string
	MarkedText            string
	MarkedBackground      string
	FooterText            string
	FooterBackground      string
	FooterNumber          string
	HeaderText            string
	HeaderBackground      string
	HeaderHidden          bool
	ResultRowNumber       string
	ResultRowDirectory    string
	// CurrentItemNameMaxLen bounds the path shown by the progress modal; 0
	// keeps the default
	CurrentItemNameMaxLen int
	ShowDiskProgressBar   bool
	OldSizeBar            bool
	ShowBarPercentage     bool
	ShowItemCountBar      bool
}

// Export configures the JSON export (--output-file).
type Export struct {
	Attributes fs.JSONAttributes
}

// Web configures the web UI.
type Web struct {
	// Listen is the address to listen on; empty for a free port on localhost
	Listen      string
	OpenBrowser bool
	Browser     string
	Cleanup     cleanup.Config
}
