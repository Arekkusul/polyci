package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Arekkusul/polyci/internal/config"
	"github.com/Arekkusul/polyci/internal/detect"
)

func names(steps []Step) []string {
	var n []string
	for _, s := range steps {
		n = append(n, s.Name)
	}
	return n
}

func joinCmd(steps []Step, name string) string {
	for _, s := range steps {
		if s.Name == name {
			return strings.Join(s.Command, " ")
		}
	}
	return ""
}

func TestGoPipeline(t *testing.T) {
	steps := Build(t.TempDir(), []detect.Stack{detect.Go}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "gofmt,vet,build,test" {
		t.Errorf("go steps = %s", got)
	}
	for _, s := range steps {
		if s.Name == "gofmt" && !s.FailOnOutput {
			t.Error("gofmt step should be FailOnOutput")
		}
	}
}

func TestRustPipeline(t *testing.T) {
	steps := Build(t.TempDir(), []detect.Stack{detect.Rust}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "fmt,clippy,build,test" {
		t.Errorf("rust steps = %s", got)
	}
}

func TestPythonUsesVenvAndRuffWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".venv", "bin"), 0o755)
	os.WriteFile(filepath.Join(dir, ".venv", "bin", "python"), []byte("x"), 0o755)
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.ruff]\n"), 0o644)
	steps := Build(dir, []detect.Stack{detect.Python}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "ruff,pytest" {
		t.Errorf("python steps = %s", got)
	}
	if c := joinCmd(steps, "pytest"); c != ".venv/bin/python -m pytest" {
		t.Errorf("pytest cmd = %q (should use venv interpreter)", c)
	}
}

func TestPythonNoRuffSectionSkipsRuff(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\n"), 0o644)
	steps := Build(dir, []detect.Stack{detect.Python}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "pytest" {
		t.Errorf("expected only pytest, got %s", got)
	}
	if c := joinCmd(steps, "pytest"); c != "python3 -m pytest" {
		t.Errorf("pytest should fall back to python3, got %q", c)
	}
}

func TestPythonMypyFromDetectedPackage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.mypy]\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "app"), 0o755)
	os.WriteFile(filepath.Join(dir, "app", "__init__.py"), nil, 0o644)
	steps := Build(dir, []detect.Stack{detect.Python}, config.Config{})
	if c := joinCmd(steps, "mypy"); c != "python3 -m mypy app" {
		t.Errorf("mypy target should be detected package, got %q", c)
	}
}

func TestNodeStepsFromScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"lint":"eslint .","test":"vitest"}}`), 0o644)
	steps := Build(dir, []detect.Stack{detect.Node}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "install,lint,test" {
		t.Errorf("node steps = %s", got)
	}
	if c := joinCmd(steps, "install"); c != "npm install" {
		t.Errorf("no lockfile should use npm install, got %q", c)
	}
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0o644)
	steps = Build(dir, []detect.Stack{detect.Node}, config.Config{})
	if c := joinCmd(steps, "install"); c != "npm ci" {
		t.Errorf("lockfile should use npm ci, got %q", c)
	}
}

func TestConfigOverrideSkipOnlyAndExtra(t *testing.T) {
	base := []detect.Stack{detect.Rust}
	cfg := config.Config{
		Override: map[string][]string{"test": {"cargo", "nextest", "run"}},
		Skip:     []string{"clippy"},
		Steps:    []config.RawStep{{Name: "smoke", Command: []string{"bash", "smoke.sh"}}},
	}
	steps := Build(t.TempDir(), base, cfg)
	if got := strings.Join(names(steps), ","); got != "fmt,build,test,smoke" {
		t.Errorf("steps = %s", got)
	}
	if c := joinCmd(steps, "test"); c != "cargo nextest run" {
		t.Errorf("override not applied: %q", c)
	}
}

func TestConfigOnlyKeepsListed(t *testing.T) {
	cfg := config.Config{Only: []string{"test"}}
	steps := Build(t.TempDir(), []detect.Stack{detect.Go}, cfg)
	if got := strings.Join(names(steps), ","); got != "test" {
		t.Errorf("only should keep just test, got %s", got)
	}
}

func TestPolyglotConcatenates(t *testing.T) {
	steps := Build(t.TempDir(), []detect.Stack{detect.Go, detect.Swift}, config.Config{})
	if got := strings.Join(names(steps), ","); got != "gofmt,vet,build,test,build" {
		t.Errorf("polyglot steps = %s", got)
	}
}
