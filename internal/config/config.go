// Package config loads an optional .polyci.json that tweaks the default pipeline for a
// project's quirks — extra env, a specific Python interpreter, a mypy target, skipping or
// restricting steps, overriding a step's command, or appending project-specific steps.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the config file polyci looks for in the project root.
const FileName = ".polyci.json"

// RawStep is a user-declared extra step.
type RawStep struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

// Config mirrors .polyci.json. All fields are optional.
type Config struct {
	Env        map[string]string   `json:"env"`         // extra environment for every step
	PythonBin  string              `json:"python_bin"`  // interpreter for python steps (default: .venv/bin/python or python3)
	MypyTarget string              `json:"mypy_target"` // enables a mypy step against this path
	Skip       []string            `json:"skip"`        // step names to drop
	Only       []string            `json:"only"`        // if set, keep only these step names
	Override   map[string][]string `json:"override"`    // step name -> replacement command (argv)
	Steps      []RawStep           `json:"steps"`       // extra steps appended after the defaults
}

// Load reads dir/.polyci.json. A missing file yields a zero Config and no error.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", FileName, err)
	}
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid %s: %w", FileName, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("invalid %s: %w", FileName, err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	for name, cmd := range c.Override {
		if len(cmd) == 0 {
			return fmt.Errorf("override %q has an empty command", name)
		}
	}
	for i, s := range c.Steps {
		if s.Name == "" {
			return fmt.Errorf("steps[%d] is missing a name", i)
		}
		if len(s.Command) == 0 {
			return fmt.Errorf("step %q has an empty command", s.Name)
		}
	}
	return nil
}
