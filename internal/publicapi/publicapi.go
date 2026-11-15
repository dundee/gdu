// Package publicapi holds the test guarding gdu's public Go API, see ADR 6.
package publicapi

// Packages are the packages other modules may import. Their exported
// identifiers must not use types from internal packages, which other modules
// cannot name.
var Packages = []string{
	"github.com/dundee/gdu/v6/pkg/analyze",
	"github.com/dundee/gdu/v6/pkg/analyze/badger",
	"github.com/dundee/gdu/v6/pkg/analyze/sqlite",
	"github.com/dundee/gdu/v6/pkg/fs",
	"github.com/dundee/gdu/v6/pkg/device",
	"github.com/dundee/gdu/v6/cmd/gdu/app",
	"github.com/dundee/gdu/v6/build",
}
