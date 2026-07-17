package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJSONAttributes(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		included []string
		excluded []string
		valid    bool
	}{
		{name: "default", valid: true, included: []string{"asize", "dsize", "mtime", "notreg"}},
		{name: "selected", value: "asize, dsize", valid: true, included: []string{"asize", "dsize"}, excluded: []string{"mtime", "notreg"}},
		{name: "name", value: "name", valid: true, excluded: []string{"asize", "dsize", "mtime", "notreg"}},
		{name: "unknown", value: "size", valid: false},
		{name: "empty selection", value: "asize,", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attributes, err := ParseJSONAttributes(test.value)
			if !test.valid {
				assert.ErrorContains(t, err, "unknown JSON output attribute")
				return
			}

			assert.NoError(t, err)
			for _, attribute := range test.included {
				assert.True(t, attributes.Includes(attribute))
			}
			for _, attribute := range test.excluded {
				assert.False(t, attributes.Includes(attribute))
			}
		})
	}
}
