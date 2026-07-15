// Package detect identifies which language stacks a project uses, by looking for
// well-known marker files. A project can match more than one stack (polyglot repos).
package detect

import (
	"os"
	"path/filepath"
)

// Stack is a supported language/build ecosystem.
type Stack string

const (
	Go     Stack = "go"
	Python Stack = "python"
	Rust   Stack = "rust"
	Node   Stack = "node"
	Swift  Stack = "swift"
	CMake  Stack = "cmake"
)

// order fixes the detection/report order so output is deterministic.
var order = []Stack{Go, Python, Rust, Node, Swift, CMake}

// markers lists the files whose presence indicates a stack (any one is enough).
var markers = map[Stack][]string{
	Go:     {"go.mod"},
	Python: {"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"},
	Rust:   {"Cargo.toml"},
	Node:   {"package.json"},
	Swift:  {"Package.swift"},
	CMake:  {"CMakeLists.txt"},
}

// Detect returns the stacks found directly in dir, in a stable order.
func Detect(dir string) []Stack {
	var found []Stack
	for _, s := range order {
		for _, m := range markers[s] {
			if fileExists(filepath.Join(dir, m)) {
				found = append(found, s)
				break
			}
		}
	}
	return found
}

// Markers returns the marker files that identify a stack (for docs/help).
func Markers(s Stack) []string { return markers[s] }

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
