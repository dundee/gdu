package common_test

import (
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestCreateIgnorePattern(t *testing.T) {
	re, err := common.CreateIgnorePattern([]string{"[abc]+"})

	assert.Nil(t, err)
	assert.True(t, re.Match([]byte("aa")))
}

func TestCreateIgnorePatternWithErr(t *testing.T) {
	re, err := common.CreateIgnorePattern([]string{"[[["})

	assert.NotNil(t, err)
	assert.Nil(t, re)
}

func TestEmptyIgnore(t *testing.T) {
	ui := &common.UI{}
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("abc", "/abc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPath(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByPattern(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("aaa", "/aaa"))
	assert.True(t, shouldBeIgnored("aaa", "/aaabc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreHidden(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPathAndHidden(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPathAndPattern(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored("aabc", "/aabc"))
	assert.True(t, shouldBeIgnored("ccc", "/ccc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByPatternAndHidden(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abbc", "/abbc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAll(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored("aabc", "/aabc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}
