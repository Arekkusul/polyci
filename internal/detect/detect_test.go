package detect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectSingleStack(t *testing.T) {
	cases := map[string]Stack{
		"go.mod":           Go,
		"pyproject.toml":   Python,
		"requirements.txt": Python,
		"Cargo.toml":       Rust,
		"package.json":     Node,
		"Package.swift":    Swift,
		"CMakeLists.txt":   CMake,
	}
	for marker, want := range cases {
		dir := t.TempDir()
		touch(t, dir, marker)
		got := Detect(dir)
		if len(got) != 1 || got[0] != want {
			t.Errorf("marker %s: got %v, want [%s]", marker, got, want)
		}
	}
}

func TestDetectPolyglotIsOrdered(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "package.json")
	touch(t, dir, "go.mod")
	touch(t, dir, "Cargo.toml")
	got := Detect(dir)
	want := []Stack{Go, Rust, Node} // fixed order, not insertion order
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDetectNoneOnEmptyDir(t *testing.T) {
	if got := Detect(t.TempDir()); len(got) != 0 {
		t.Errorf("empty dir: got %v, want none", got)
	}
}

func TestDetectIgnoresDirectoryNamedLikeMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Detect(dir); len(got) != 0 {
		t.Errorf("directory marker should not count: got %v", got)
	}
}
