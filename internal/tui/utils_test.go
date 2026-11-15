package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUsageGraph(t *testing.T) {
	assert.Equal(t, "           \u258F", getUsageGraph(0))
	assert.Equal(t, " █         \u258F", getUsageGraph(10))
	assert.Equal(t, " ██        \u258F", getUsageGraph(20))
	assert.Equal(t, " ███       \u258F", getUsageGraph(30))
	assert.Equal(t, " ████      \u258F", getUsageGraph(40))
	assert.Equal(t, " █████     \u258F", getUsageGraph(50))
	assert.Equal(t, " ██████    \u258F", getUsageGraph(60))
	assert.Equal(t, " ███████   \u258F", getUsageGraph(70))
	assert.Equal(t, " ████████  \u258F", getUsageGraph(80))
	assert.Equal(t, " █████████ \u258F", getUsageGraph(90))
	assert.Equal(t, " ██████████\u258F", getUsageGraph(100))

	assert.Equal(t, " █         \u258F", getUsageGraph(11))
	assert.Equal(t, " █▏        \u258F", getUsageGraph(12))
	assert.Equal(t, " █▎        \u258F", getUsageGraph(13))
	assert.Equal(t, " █▍        \u258F", getUsageGraph(14))
	assert.Equal(t, " █▌        \u258F", getUsageGraph(15))
	assert.Equal(t, " █▌        \u258F", getUsageGraph(16))
	assert.Equal(t, " █▋        \u258F", getUsageGraph(17))
	assert.Equal(t, " █▊        \u258F", getUsageGraph(18))
	assert.Equal(t, " █▉        \u258F", getUsageGraph(19))
}
