package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "git.eiac.dev/tools/denar"

// TestDependencyRule enforces adapter -> usecase -> domain: inner layers may
// import only the standard library and the layers listed for them.
func TestDependencyRule(t *testing.T) {
	allowed := map[string][]string{
		"domain":  {module + "/internal/domain"},
		"usecase": {module + "/internal/domain", module + "/internal/usecase"},
	}
	for layer, prefixes := range allowed {
		err := filepath.WalkDir(layer, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if !importAllowed(p, prefixes) {
					t.Errorf("%s imports %q, which the %s layer may not depend on", path, p, layer)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", layer, err)
		}
	}
}

// TestAdaptersStayApart keeps the adapters independent of each other: they
// meet only in the composition root. In particular the REST adapter must
// not reach into the SQLite one.
func TestAdaptersStayApart(t *testing.T) {
	adapters, err := os.ReadDir("adapter")
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) < 2 {
		t.Fatalf("found %d adapters, want several", len(adapters))
	}
	for _, own := range adapters {
		if !own.IsDir() {
			continue
		}
		ownPrefix := module + "/internal/adapter/" + own.Name()
		err := filepath.WalkDir(filepath.Join("adapter", own.Name()), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(p, module+"/internal/adapter/") && p != ownPrefix && !strings.HasPrefix(p, ownPrefix+"/") {
					t.Errorf("%s imports %q, another adapter", path, p)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking adapter/%s: %v", own.Name(), err)
		}
	}
}

func importAllowed(path string, prefixes []string) bool {
	// Standard library packages have no dot in their first path element.
	if first, _, _ := strings.Cut(path, "/"); !strings.Contains(first, ".") {
		return true
	}
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
