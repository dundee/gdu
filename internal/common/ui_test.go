// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	res := FormatNumber(1234567890)
	assert.Equal(t, "1,234,567,890", res)
}

func TestSetFollowSymlinks(t *testing.T) {
	ui := UI{
		Analyzer: &MockedAnalyzer{},
	}
	ui.SetFollowSymlinks(true)

	assert.Equal(t, true, ui.Analyzer.(*MockedAnalyzer).FollowSymlinks)
}

func TestSetShowAnnexedSize(t *testing.T) {
	ui := UI{
		Analyzer: &MockedAnalyzer{},
	}
	ui.SetShowAnnexedSize(true)

	assert.Equal(t, true, ui.Analyzer.(*MockedAnalyzer).ShowAnnexedSize)
}

func TestSetEnableArchiveBrowsing(t *testing.T) {
	ui := UI{
		Analyzer: &MockedAnalyzer{},
	}
	ui.SetArchiveBrowsing(true)

	assert.Equal(t, true, ui.Analyzer.(*MockedAnalyzer).ArchiveBrowsing)
}

type MockedAnalyzer struct {
	FollowSymlinks  bool
	ShowAnnexedSize bool
	ArchiveBrowsing bool
}

// SetFileTypeFilter sets the file type filter function
func (a *MockedAnalyzer) SetFileTypeFilter(filter ShouldFileBeFiltered) {
	// Mock implementation - do nothing
}

// AnalyzeDir returns dir with files with different size exponents
func (a *MockedAnalyzer) AnalyzeDir(
	path string, ignore ShouldDirBeIgnored, fileTypeFilter ShouldFileBeFiltered, enableGC bool,
) fs.Item {
	return nil
}

// GetProgressChan returns always Done
func (a *MockedAnalyzer) GetProgressChan() chan CurrentProgress {
	return make(chan CurrentProgress)
}

// GetDone returns always Done
func (a *MockedAnalyzer) GetDone() SignalGroup {
	c := make(SignalGroup)
	defer c.Broadcast()
	return c
}

// ResetProgress does nothing
func (a *MockedAnalyzer) ResetProgress() {}

// SetFollowSymlinks does nothing
func (a *MockedAnalyzer) SetFollowSymlinks(v bool) {
	a.FollowSymlinks = v
}

// SetShowAnnexedSize does nothing
func (a *MockedAnalyzer) SetShowAnnexedSize(v bool) {
	a.ShowAnnexedSize = v
}

// SetTimeFilter does nothing
func (a *MockedAnalyzer) SetTimeFilter(timeFilter TimeFilter) {}

// SetArchiveBrowsing sets EnableArchiveBrowsing
func (a *MockedAnalyzer) SetArchiveBrowsing(v bool) {
	a.ArchiveBrowsing = v
}
