// Package plan turns a set of detected stacks into an ordered list of gate steps
// (fmt → lint → typecheck → build → test), then applies the project's config overrides.
package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Arekkusul/polyci/internal/config"
	"github.com/Arekkusul/polyci/internal/detect"
)

// Step is one gate command.
type Step struct {
	Name         string
	Command      []string
	Optional     bool // skip (don't fail) when the executable isn't on PATH
	FailOnOutput bool // fail if the command writes anything to stdout (e.g. `gofmt -l`)
}

// Build assembles the full pipeline for dir given the detected stacks and config.
func Build(dir string, stacks []detect.Stack, cfg config.Config) []Step {
	var steps []Step
	for _, s := range stacks {
		steps = append(steps, defaults(dir, s, cfg)...)
	}
	steps = applyConfig(steps, cfg)
	return steps
}

func defaults(dir string, s detect.Stack, cfg config.Config) []Step {
	switch s {
	case detect.Go:
		return []Step{
			{Name: "gofmt", Command: []string{"gofmt", "-l", "."}, FailOnOutput: true},
			{Name: "vet", Command: []string{"go", "vet", "./..."}},
			{Name: "build", Command: []string{"go", "build", "./..."}},
			{Name: "test", Command: []string{"go", "test", "./..."}},
		}
	case detect.Rust:
		return []Step{
			{Name: "fmt", Command: []string{"cargo", "fmt", "--all", "--", "--check"}},
			{Name: "clippy", Command: []string{"cargo", "clippy", "--all-targets", "--", "-D", "warnings"}},
			{Name: "build", Command: []string{"cargo", "build"}},
			{Name: "test", Command: []string{"cargo", "test"}},
		}
	case detect.Python:
		return pythonSteps(dir, cfg)
	case detect.Node:
		return nodeSteps(dir)
	case detect.Swift:
		return []Step{{Name: "build", Command: []string{"swift", "build"}}}
	case detect.CMake:
		return []Step{
			{Name: "cmake", Command: []string{"cmake", "-S", ".", "-B", "build"}},
			{Name: "build", Command: []string{"cmake", "--build", "build"}},
			{Name: "ctest", Command: []string{"ctest", "--test-dir", "build", "--output-on-failure"}},
		}
	}
	return nil
}

func pythonSteps(dir string, cfg config.Config) []Step {
	interp := cfg.PythonBin
	if interp == "" {
		if fileExists(filepath.Join(dir, ".venv", "bin", "python")) {
			interp = ".venv/bin/python"
		} else {
			interp = "python3"
		}
	}
	pyproject := readFile(filepath.Join(dir, "pyproject.toml"))
	var steps []Step
	if strings.Contains(pyproject, "[tool.ruff") {
		steps = append(steps, Step{Name: "ruff", Command: []string{interp, "-m", "ruff", "check", "."}})
	}
	target := cfg.MypyTarget
	if target == "" && strings.Contains(pyproject, "[tool.mypy]") {
		if pkg := pyPackageDir(dir); pkg != "" {
			target = pkg
		} else {
			target = "."
		}
	}
	if target != "" {
		steps = append(steps, Step{Name: "mypy", Command: []string{interp, "-m", "mypy", target}})
	}
	steps = append(steps, Step{Name: "pytest", Command: []string{interp, "-m", "pytest"}})
	return steps
}

func nodeSteps(dir string) []Step {
	install := []string{"npm", "install"}
	if fileExists(filepath.Join(dir, "package-lock.json")) {
		install = []string{"npm", "ci"}
	}
	steps := []Step{{Name: "install", Command: install}}
	scripts := packageScripts(dir)
	for _, name := range []string{"lint", "typecheck", "build"} {
		if _, ok := scripts[name]; ok {
			steps = append(steps, Step{Name: name, Command: []string{"npm", "run", name}})
		}
	}
	if _, ok := scripts["test"]; ok {
		steps = append(steps, Step{Name: "test", Command: []string{"npm", "test"}})
	}
	return steps
}

// applyConfig layers override → extra steps → only → skip onto the default pipeline.
func applyConfig(steps []Step, cfg config.Config) []Step {
	for i := range steps {
		if repl, ok := cfg.Override[steps[i].Name]; ok {
			steps[i].Command = repl
			steps[i].FailOnOutput = false // an override replaces the whole command semantics
		}
	}
	for _, s := range cfg.Steps {
		steps = append(steps, Step{Name: s.Name, Command: s.Command})
	}
	if len(cfg.Only) > 0 {
		keep := set(cfg.Only)
		steps = filter(steps, func(s Step) bool { return keep[s.Name] })
	}
	if len(cfg.Skip) > 0 {
		drop := set(cfg.Skip)
		steps = filter(steps, func(s Step) bool { return !drop[s.Name] })
	}
	return steps
}

// ---- helpers ----

func pyPackageDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == "tests" || name == "test" || strings.HasPrefix(name, ".") {
			continue
		}
		if fileExists(filepath.Join(dir, name, "__init__.py")) {
			return name
		}
	}
	return ""
}

func packageScripts(dir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	return pkg.Scripts
}

func filter(steps []Step, keep func(Step) bool) []Step {
	out := steps[:0:0]
	for _, s := range steps {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}

func set(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func readFile(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(data)
}
