// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	res := FormatNumber(1234567890)
	assert.Equal(t, "1,234,567,890", res)
}

// The scan settings are kept by the UI and passed to each scan, whichever
// analyzer is set, before or after them.
func TestScanSettingsAreScanOptions(t *testing.T) {
	ui := UI{}
	ui.SetFollowSymlinks(true)
	ui.SetShowAnnexedSize(true)
	ui.SetExclusiveSize(true)
	ui.SetArchiveBrowsing(true)
	ui.SetTimeFilter(func(time.Time) bool { return true })
	ui.SetAnalyzer(&MockedAnalyzer{})

	opts := ui.ScanOptions()

	assert.True(t, opts.FollowSymlinks)
	assert.True(t, opts.AnnexedSize)
	assert.True(t, opts.ExclusiveSize)
	assert.True(t, opts.ArchiveBrowsing)
	assert.NotNil(t, opts.TimeFilter)
	assert.NotNil(t, opts.IgnoreDir)
	assert.Nil(t, opts.IgnoreFile, "no file is filtered by its name")
	assert.True(t, opts.FilteringFiles())
	assert.True(t, ui.IsFilteringFiles())
}

func TestScanOptionsByDefault(t *testing.T) {
	opts := (&UI{}).ScanOptions()

	assert.False(t, opts.FollowSymlinks || opts.AnnexedSize || opts.ExclusiveSize || opts.ArchiveBrowsing)
	assert.False(t, opts.FilteringFiles())
}

func TestSetAnalyzer(t *testing.T) {
	ui := UI{}
	a := &MockedAnalyzer{}
	ui.SetAnalyzer(a)
	assert.Equal(t, a, ui.Analyzer)
}

type MockedAnalyzer struct{}

// AnalyzeDir returns nothing
func (a *MockedAnalyzer) AnalyzeDir(context.Context, string, ScanOptions) fs.Item {
	return nil
}

// GetProgress returns empty progress
func (a *MockedAnalyzer) GetProgress() CurrentProgress {
	return CurrentProgress{}
}

func TestSetBlockSizeFromEnvironment(t *testing.T) {
	t.Run("BLOCK_SIZE takes precedence", func(t *testing.T) {
		t.Setenv("BLOCK_SIZE", "1K")
		t.Setenv("BLOCKSIZE", "1MB")
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		formatted, ok := ui.FormatBlockSize(1025)
		assert.True(t, ok)
		assert.Equal(t, "2", formatted)
	})

	t.Run("BLOCKSIZE fallback", func(t *testing.T) {
		unsetBlockSizeEnvironment(t)
		t.Setenv("BLOCKSIZE", "2kB")
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		formatted, ok := ui.FormatBlockSize(2001)
		assert.True(t, ok)
		assert.Equal(t, "2", formatted)
	})

	t.Run("human readable", func(t *testing.T) {
		t.Setenv("BLOCK_SIZE", "human-readable")
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		_, ok := ui.FormatBlockSize(1)
		assert.False(t, ok)
		assert.False(t, ui.UseSIPrefix)
	})

	t.Run("si", func(t *testing.T) {
		t.Setenv("BLOCK_SIZE", "si")
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		assert.True(t, ui.UseSIPrefix)
	})

	t.Run("invalid value", func(t *testing.T) {
		t.Setenv("BLOCK_SIZE", "invalid")
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		_, ok := ui.FormatBlockSize(1)
		assert.False(t, ok)
	})

	t.Run("absent", func(t *testing.T) {
		unsetBlockSizeEnvironment(t)
		ui := &UI{}
		ui.SetBlockSizeFromEnvironment()

		_, ok := ui.FormatBlockSize(1)
		assert.False(t, ok)
	})
}

func TestFormatBlockSize(t *testing.T) {
	ui := &UI{blockSize: 1000, blockSuffix: "kB"}

	tests := []struct {
		size     int64
		expected string
	}{
		{size: -1, expected: "0kB"},
		{size: 0, expected: "0kB"},
		{size: 1000, expected: "1kB"},
		{size: 1001, expected: "2kB"},
	}

	for _, test := range tests {
		formatted, ok := ui.FormatBlockSize(test.size)
		assert.True(t, ok)
		assert.Equal(t, test.expected, formatted)
	}
}

func TestParseBlockSize(t *testing.T) {
	tests := []struct {
		value  string
		size   int64
		suffix string
		valid  bool
	}{
		{value: "1", size: 1, valid: true},
		{value: "k", size: 1 << 10, suffix: "k", valid: true},
		{value: "kB", size: 1e3, suffix: "kB", valid: true},
		{value: "1K", size: 1 << 10, valid: true},
		{value: "1M", size: 1 << 20, valid: true},
		{value: "1MB", size: 1e6, valid: true},
		{value: "1G", size: 1 << 30, valid: true},
		{value: "1GB", size: 1e9, valid: true},
		{value: "1T", size: 1 << 40, valid: true},
		{value: "1TB", size: 1e12, valid: true},
		{value: "1P", size: 1 << 50, valid: true},
		{value: "1PB", size: 1e15, valid: true},
		{value: "1E", size: 1 << 60, valid: true},
		{value: "1EB", size: 1e18, valid: true},
		{value: "", valid: false},
		{value: "0", valid: false},
		{value: "-1", valid: false},
		{value: "1Z", valid: false},
		{value: "8E", valid: false},
		{value: "999999999999999999999", valid: false},
		{value: "'1", valid: false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			size, suffix, ok := parseBlockSize(test.value)
			assert.Equal(t, test.valid, ok)
			if ok {
				assert.Equal(t, test.size, size)
				assert.Equal(t, test.suffix, suffix)
			}
		})
	}
}

func unsetBlockSizeEnvironment(t *testing.T) {
	t.Helper()
	blockSize, blockSizeSet := os.LookupEnv("BLOCK_SIZE")
	legacyBlockSize, legacyBlockSizeSet := os.LookupEnv("BLOCKSIZE")
	os.Unsetenv("BLOCK_SIZE")
	os.Unsetenv("BLOCKSIZE")
	t.Cleanup(func() {
		if blockSizeSet {
			os.Setenv("BLOCK_SIZE", blockSize)
		} else {
			os.Unsetenv("BLOCK_SIZE")
		}
		if legacyBlockSizeSet {
			os.Setenv("BLOCKSIZE", legacyBlockSize)
		} else {
			os.Unsetenv("BLOCKSIZE")
		}
	})
}
