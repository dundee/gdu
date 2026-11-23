//go:build windows || plan9

package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOtherGetMountsReturnsNoMountsWithoutError(t *testing.T) {
	mounts, err := Getter.GetMounts()

	assert.NoError(t, err)
	assert.Empty(t, mounts)
}

func TestOtherGetDevicesInfoIsNotSupported(t *testing.T) {
	_, err := Getter.GetDevicesInfo()

	assert.Error(t, err)
}
