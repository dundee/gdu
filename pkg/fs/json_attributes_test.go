package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONAttributesIncludes(t *testing.T) {
	t.Run("nil set includes every attribute", func(t *testing.T) {
		var attributes JSONAttributes
		assert.True(t, attributes.Includes("asize"))
		assert.True(t, attributes.Includes("anything"))
	})

	t.Run("empty set includes nothing", func(t *testing.T) {
		attributes := JSONAttributes{}
		assert.False(t, attributes.Includes("asize"))
	})

	t.Run("selected set includes only its members", func(t *testing.T) {
		attributes := JSONAttributes{"asize": {}, "dsize": {}}
		assert.True(t, attributes.Includes("asize"))
		assert.True(t, attributes.Includes("dsize"))
		assert.False(t, attributes.Includes("items"))
	})
}
