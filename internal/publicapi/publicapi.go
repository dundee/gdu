// Package publicapi holds the test guarding gdu's public Go API, see ADR 6.
package publicapi

// Packages are the packages other modules may import. Their exported
// identifiers must not use types from internal packages, which other modules
// cannot name.
var Packages = []string{
	"github.com/dundee/gdu/v5/pkg/analyze",
	"github.com/dundee/gdu/v5/pkg/fs",
	"github.com/dundee/gdu/v5/pkg/device",
	"github.com/dundee/gdu/v5/cmd/gdu/app",
	"github.com/dundee/gdu/v5/build",
}
