package main

import (
	"strings"

	"github.com/Arekkusul/polyci/internal/detect"
)

// workflowYAML generates a GitHub Actions workflow tailored to the detected stacks:
// it sets up the needed toolchains, installs polyci, and runs the gates. Go is always
// set up because installing polyci needs it. Swift forces a macOS runner.
func workflowYAML(stacks []detect.Stack) string {
	has := map[detect.Stack]bool{}
	for _, s := range stacks {
		has[s] = true
	}
	runner := "ubuntu-latest"
	if has[detect.Swift] {
		runner = "macos-14"
	}

	var b strings.Builder
	b.WriteString("name: CI\n")
	b.WriteString("on: [push, pull_request]\n")
	b.WriteString("jobs:\n")
	b.WriteString("  ci:\n")
	b.WriteString("    runs-on: " + runner + "\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	b.WriteString("      - uses: actions/setup-go@v5\n")
	b.WriteString("        with:\n")
	b.WriteString("          go-version: \"1.26\"\n")

	if has[detect.Python] {
		b.WriteString("      - uses: actions/setup-python@v5\n")
		b.WriteString("        with:\n")
		b.WriteString("          python-version: \"3.13\"\n")
		b.WriteString("      - run: pip install pytest ruff mypy\n")
		b.WriteString("      - run: pip install -e . || true   # adjust to your project's dependencies\n")
	}
	if has[detect.Node] {
		b.WriteString("      - uses: actions/setup-node@v4\n")
		b.WriteString("        with:\n")
		b.WriteString("          node-version: \"20\"\n")
	}
	if has[detect.Rust] {
		b.WriteString("      - run: rustup component add clippy rustfmt\n")
	}
	if has[detect.CMake] && runner == "ubuntu-latest" {
		b.WriteString("      - run: sudo apt-get update && sudo apt-get install -y cmake   # add your build deps\n")
	}

	b.WriteString("      - name: install polyci\n")
	b.WriteString("        run: go install github.com/Arekkusul/polyci@latest\n")
	b.WriteString("      - name: run gates\n")
	b.WriteString("        run: polyci run\n")
	return b.String()
}
