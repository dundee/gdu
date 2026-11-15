package analyze_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// The largest entries of a directory, as another program would list them
// with gdu's public API alone.
func ExampleParallelAnalyzer_AnalyzeDir() {
	root, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(root)
	_ = os.MkdirAll(filepath.Join(root, "logs"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "logs", "app.log"), []byte(strings.Repeat("x", 3000)), 0o600)
	_ = os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello"), 0o600)
	_ = os.MkdirAll(filepath.Join(root, ".cache"), 0o755)

	opts := analyze.Options{
		// leave out hidden directories
		IgnoreDir: func(name, _ string) bool { return strings.HasPrefix(name, ".") },
	}
	tree := analyze.CreateAnalyzer().AnalyzeDir(context.Background(), root, opts)
	tree.UpdateStats(opts.FilteringFiles())

	for entry := range tree.GetFiles(fs.SortByApparentSize, fs.SortDesc) {
		fmt.Println(entry.GetName(), entry.GetSize(), entry.IsDir())
	}
	// Output:
	// logs 3000 true
	// notes.txt 5 false
}
