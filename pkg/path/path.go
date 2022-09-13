package path

import "strings"

// ShortenPath removes the last but one path components to fit into maxLen
func ShortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	res := ""
	parts := strings.SplitAfter(path, "/")
	curLen := len(parts[len(parts)-1]) // count lenght of last part for start

	for _, part := range parts[:len(parts)-1] {
		curLen += len(part)
		if curLen > maxLen {
			res += ".../"
			break
		}
		res += part
	}

	res += parts[len(parts)-1]
	return res
}
