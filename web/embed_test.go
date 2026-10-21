package web

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

func TestDistIsRootedInDist(t *testing.T) {
	// The placeholder keeps go:embed satisfied on a checkout without a web build.
	if _, err := fs.Stat(Dist(), ".gitkeep"); err != nil {
		t.Fatalf("Dist() has no .gitkeep: %v", err)
	}
	if _, err := fs.Stat(Dist(), "dist"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Dist() exposes a nested dist directory, err = %v", err)
	}
}

func TestDistMatchesWebBuild(t *testing.T) {
	_, err := os.Stat("dist/index.html")
	built := err == nil

	entries, err := fs.ReadDir(Dist(), ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if !built {
		if len(entries) != 1 || entries[0].Name() != ".gitkeep" {
			t.Errorf("without a web build Dist() should hold only .gitkeep, got %v", entries)
		}
		return
	}
	index, err := fs.ReadFile(Dist(), "index.html")
	if err != nil {
		t.Fatalf("web is built but Dist() has no index.html: %v", err)
	}
	if len(index) == 0 {
		t.Error("embedded index.html is empty")
	}
}
