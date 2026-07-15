package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFileIsZeroConfig(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.PythonBin != "" || len(cfg.Skip) != 0 {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

func TestLoadParsesFields(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{
		"env": {"DYLD_LIBRARY_PATH": "/opt/homebrew/opt/expat/lib"},
		"python_bin": ".venv/bin/python",
		"mypy_target": "app",
		"skip": ["clippy"],
		"override": {"test": ["swift", "run", "kit-tests"]},
		"steps": [{"name": "smoke", "command": ["bash", "smoke.sh"]}]
	}`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env["DYLD_LIBRARY_PATH"] == "" || cfg.PythonBin != ".venv/bin/python" || cfg.MypyTarget != "app" {
		t.Errorf("fields not parsed: %+v", cfg)
	}
	if len(cfg.Skip) != 1 || cfg.Skip[0] != "clippy" {
		t.Errorf("skip not parsed: %+v", cfg.Skip)
	}
	if got := cfg.Override["test"]; len(got) != 3 || got[0] != "swift" {
		t.Errorf("override not parsed: %+v", got)
	}
	if len(cfg.Steps) != 1 || cfg.Steps[0].Name != "smoke" {
		t.Errorf("steps not parsed: %+v", cfg.Steps)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"bogus": true}`)
	if _, err := Load(dir); err == nil {
		t.Error("expected error on unknown field")
	}
}

func TestLoadRejectsEmptyOverrideAndStep(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"override": {"test": []}}`)
	if _, err := Load(dir); err == nil {
		t.Error("expected error on empty override command")
	}
	write(t, dir, `{"steps": [{"name": "x", "command": []}]}`)
	if _, err := Load(dir); err == nil {
		t.Error("expected error on empty step command")
	}
	write(t, dir, `{"steps": [{"command": ["ls"]}]}`)
	if _, err := Load(dir); err == nil {
		t.Error("expected error on nameless step")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{not json`)
	if _, err := Load(dir); err == nil {
		t.Error("expected error on malformed json")
	}
}
