package fs

// JSONAttributes selects optional attributes written to an analysis export.
// A nil set preserves the complete legacy export.
type JSONAttributes map[string]struct{}

// Includes reports whether an optional JSON attribute should be written.
func (attributes JSONAttributes) Includes(attribute string) bool {
	if attributes == nil {
		return true
	}
	_, ok := attributes[attribute]
	return ok
}
