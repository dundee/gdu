package publicapi

import (
	"bufio"
	"bytes"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Every package of the module outside internal/ is public, and the public
// packages expose no types of internal packages.
func TestPublicAPIUsesNoInternalTypes(t *testing.T) {
	exports := exportData(t)
	imp := importer.ForCompiler(token.NewFileSet(), "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})

	for _, path := range Packages {
		pkg, err := imp.Import(path)
		if err != nil {
			t.Fatalf("importing %s: %v", path, err)
		}
		for _, name := range pkg.Scope().Names() {
			obj := pkg.Scope().Lookup(name)
			if !obj.Exported() {
				continue
			}
			c := checker{t: t, where: pkg.Name() + "." + name, seen: map[types.Type]bool{}}
			c.check(obj.Type(), true)
		}
	}
}

func TestEveryPackageOutsideInternalIsListed(t *testing.T) {
	out, err := exec.Command("go", "list", "github.com/dundee/gdu/v5/...").Output()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{"github.com/dundee/gdu/v5/cmd/gdu": true}
	for _, path := range Packages {
		listed[path] = true
	}
	for _, path := range strings.Fields(string(out)) {
		if !strings.Contains(path, "/internal/") && !listed[path] {
			t.Errorf("%s can be imported by other modules: move it to internal/ or add it to Packages", path)
		}
	}
}

// A program scanning directories with pkg/analyze links neither the
// databases, which pkg/analyze/badger and pkg/analyze/sqlite bring in, nor
// the terminal UI.
func TestAnalyzeLinksNoDatabaseNorUI(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/dundee/gdu/v5/pkg/analyze").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range strings.Fields(string(out)) {
		for _, heavy := range []string{"github.com/dgraph-io/", "modernc.org/", "github.com/rivo/tview", "github.com/gdamore/tcell"} {
			if strings.HasPrefix(path, heavy) {
				t.Errorf("pkg/analyze depends on %s", path)
			}
		}
	}
}

// exportData returns the files holding the compiled export data of the
// public packages and their dependencies, by import path.
func exportData(t *testing.T) map[string]string {
	t.Helper()
	args := append([]string{"list", "-export", "-deps", "-f", "{{.ImportPath}}\t{{.Export}}"}, Packages...)
	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	exports := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if path, file, ok := strings.Cut(scanner.Text(), "\t"); ok && file != "" {
			exports[path] = file
		}
	}
	return exports
}

type checker struct {
	t     *testing.T
	where string
	seen  map[types.Type]bool
}

// check reports types of internal packages used by typ. Named types of other
// packages are not looked into, as each public package is checked on its own.
func (c *checker) check(typ types.Type, declared bool) {
	if c.seen[typ] {
		return
	}
	c.seen[typ] = true

	switch typ := typ.(type) {
	case *types.Alias:
		c.check(types.Unalias(typ), declared)
	case *types.Named:
		if pkg := typ.Obj().Pkg(); pkg != nil && strings.Contains(pkg.Path(), "/internal/") {
			c.t.Errorf("%s uses %s.%s", c.where, pkg.Path(), typ.Obj().Name())
			return
		}
		if !declared {
			return
		}
		c.check(typ.Underlying(), false)
		for method := range typ.Methods() {
			if method.Exported() {
				c.check(method.Type(), false)
			}
		}
	case *types.Pointer:
		c.check(typ.Elem(), false)
	case *types.Slice:
		c.check(typ.Elem(), false)
	case *types.Array:
		c.check(typ.Elem(), false)
	case *types.Map:
		c.check(typ.Key(), false)
		c.check(typ.Elem(), false)
	case *types.Chan:
		c.check(typ.Elem(), false)
	case *types.Signature:
		for param := range typ.Params().Variables() {
			c.check(param.Type(), false)
		}
		for result := range typ.Results().Variables() {
			c.check(result.Type(), false)
		}
	case *types.Struct:
		for field := range typ.Fields() {
			if field.Exported() || field.Embedded() {
				c.check(field.Type(), false)
			}
		}
	case *types.Interface:
		for method := range typ.Methods() {
			c.check(method.Type(), false)
		}
		for embedded := range typ.EmbeddedTypes() {
			c.check(embedded, false)
		}
	}
}
