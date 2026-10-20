package api_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// generator must match the //go:generate line in doc.go.
const generator = "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0"

// generate runs the generator in a scratch directory with the committed
// configuration and returns the file it writes. The config file fixes the
// output name, so the directory is what keeps gen.go untouched.
func generate(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	cfg, err := os.ReadFile("oapi-codegen.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "oapi-codegen.yaml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := filepath.Abs("../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", generator, "-config", "oapi-codegen.yaml", spec)
	cmd.Dir = dir
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("oapi-codegen: %v\n%s", err, msg)
	}
	got, err := os.ReadFile(filepath.Join(dir, "gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestGeneratedCodeIsCurrent fails when gen.go is stale or when the
// generator's output is not reproducible. It needs the generator in the
// module cache or network access, so -short skips it.
func TestGeneratedCodeIsCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the code generator")
	}
	committed, err := os.ReadFile("gen.go")
	if err != nil {
		t.Fatal(err)
	}
	first := generate(t)
	second := generate(t)

	if !bytes.Equal(first, second) {
		t.Error("two generator runs gave different output")
	}
	if !bytes.Equal(first, committed) {
		t.Error("gen.go is out of date; run go generate ./internal/adapter/rest/api/...")
	}
}
