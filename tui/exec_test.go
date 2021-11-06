package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExec(t *testing.T) {
	err := Execute("true", []string{}, []string{})

	assert.Nil(t, err)
}
