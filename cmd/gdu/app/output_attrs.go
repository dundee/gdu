package app

import (
	"fmt"
	"strings"

	gfs "github.com/dundee/gdu/v5/pkg/fs"
)

// parseJSONAttributes parses the comma-separated --output-attrs value into the
// set of optional attributes to include in the JSON export. An empty value
// yields a nil set, which preserves the complete legacy export.
func parseJSONAttributes(value string) (gfs.JSONAttributes, error) {
	if value == "" {
		return nil, nil
	}

	attributes := make(gfs.JSONAttributes)
	for _, attribute := range strings.Split(value, ",") {
		attribute = strings.TrimSpace(attribute)
		switch attribute {
		case "name", "asize", "dsize", "items", "mtime", "notreg":
			attributes[attribute] = struct{}{}
		default:
			return nil, fmt.Errorf("unknown JSON output attribute %q", attribute)
		}
	}

	return attributes, nil
}
