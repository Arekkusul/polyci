// Command polyci detects a project's language stack(s) and runs the right CI gates
// (fmt, lint, typecheck, build, test) — locally and in CI, uniformly across projects.
//
//	polyci detect            # print detected stacks
//	polyci list              # print the gate steps that would run
//	polyci run               # run the gates (exit non-zero on failure)
//	polyci init              # write a GitHub Actions workflow tailored to this repo
//	polyci version
//
// An optional .polyci.json tunes the pipeline (env, python interpreter, mypy target,
// skip/only, per-step override, extra steps).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Arekkusul/polyci/internal/config"
	"github.com/Arekkusul/polyci/internal/detect"
	"github.com/Arekkusul/polyci/internal/plan"
	"github.com/Arekkusul/polyci/internal/run"
)

const version = "0.1.0"

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "detect":
		return cmdDetect(rest)
	case "list":
		return cmdList(rest)
	case "run":
		return cmdRun(rest)
	case "init":
		return cmdInit(rest)
	case "version", "--version", "-v":
		fmt.Println("polyci", version)
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "polyci: unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `polyci — one CI gate for every stack

usage:
  polyci detect [--dir D]
  polyci list   [--dir D]
  polyci run    [--dir D] [--only a,b] [--skip c] [--keep-going]
  polyci init   [--dir D]
  polyci version
`)
}

// loadPlan detects stacks, loads config, and builds the pipeline for dir.
func loadPlan(dir string) ([]detect.Stack, config.Config, []plan.Step, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, cfg, nil, err
	}
	stacks := detect.Detect(dir)
	steps := plan.Build(dir, stacks, cfg)
	return stacks, cfg, steps, nil
}

func cmdDetect(args []string) int {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	stacks := detect.Detect(*dir)
	if len(stacks) == 0 {
		fmt.Println("no known stack detected")
		return 0
	}
	parts := make([]string, len(stacks))
	for i, s := range stacks {
		parts[i] = string(s)
	}
	fmt.Println(strings.Join(parts, " "))
	return 0
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	stacks, _, steps, err := loadPlan(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "polyci:", err)
		return 2
	}
	if len(steps) == 0 {
		fmt.Println("no steps (no known stack detected)")
		return 0
	}
	fmt.Printf("stacks: %s\n", stacksLine(stacks))
	for _, s := range steps {
		fmt.Printf("  %-10s %s\n", s.Name, strings.Join(s.Command, " "))
	}
	return 0
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dir := fs.String("dir", ".", "project directory")
	only := fs.String("only", "", "run only these steps (comma-separated)")
	skip := fs.String("skip", "", "skip these steps (comma-separated)")
	keepGoing := fs.Bool("keep-going", false, "run all steps even if one fails")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "polyci:", err)
		return 2
	}
	mergeCSV(&cfg.Only, *only)
	mergeCSV(&cfg.Skip, *skip)
	stacks := detect.Detect(*dir)
	steps := plan.Build(*dir, stacks, cfg)
	if len(steps) == 0 {
		fmt.Fprintln(os.Stderr, "polyci: no known stack detected in", *dir)
		return 1
	}

	c := newColors()
	fmt.Printf("%spolyci%s  %s  (%d steps)\n", c.bold, c.reset, stacksLine(stacks), len(steps))
	results := run.Run(steps, run.Options{
		Dir: *dir, Env: cfg.Env, KeepGoing: *keepGoing,
		Stdout: os.Stdout, Stderr: os.Stderr,
		OnStart: func(s plan.Step) {
			fmt.Printf("\n%s▶ %s%s  %s%s%s\n", c.bold, s.Name, c.reset, c.dim, strings.Join(s.Command, " "), c.reset)
		},
	})
	printSummary(c, results)
	if run.Failures(results) > 0 {
		return 1
	}
	return 0
}

func printSummary(c colors, results []run.Result) {
	fmt.Printf("\n%s── summary ──%s\n", c.bold, c.reset)
	for _, r := range results {
		mark, col := "✓", c.green
		switch r.Status {
		case run.Fail:
			mark, col = "✗", c.red
		case run.Skip:
			mark, col = "–", c.dim
		}
		fmt.Printf("  %s%s %-10s%s %s\n", col, mark, r.Step.Name, c.reset, dur(r.Duration))
	}
	failed := run.Failures(results)
	if failed == 0 {
		fmt.Printf("%s✓ all gates passed%s\n", c.green, c.reset)
	} else {
		fmt.Printf("%s✗ %d gate(s) failed%s\n", c.red, failed, c.reset)
	}
}

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", ".", "project directory")
	force := fs.Bool("force", false, "overwrite an existing workflow")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	stacks := detect.Detect(*dir)
	if len(stacks) == 0 {
		fmt.Fprintln(os.Stderr, "polyci: no known stack detected; nothing to init")
		return 1
	}
	out := *dir + "/.github/workflows/ci.yml"
	if _, err := os.Stat(out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "polyci: %s already exists (use --force to overwrite)\n", out)
		return 1
	}
	if err := os.MkdirAll(*dir+"/.github/workflows", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "polyci:", err)
		return 1
	}
	if err := os.WriteFile(out, []byte(workflowYAML(stacks)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "polyci:", err)
		return 1
	}
	fmt.Printf("wrote %s for %s\n", out, stacksLine(stacks))
	return 0
}

func stacksLine(stacks []detect.Stack) string {
	parts := make([]string, len(stacks))
	for i, s := range stacks {
		parts[i] = string(s)
	}
	return strings.Join(parts, "+")
}

func mergeCSV(dst *[]string, csv string) {
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*dst = append(*dst, p)
		}
	}
}

func dur(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.Round(time.Millisecond).String()
}

// ---- colors ----

type colors struct{ bold, dim, red, green, reset string }

func newColors() colors {
	if os.Getenv("NO_COLOR") != "" || !isTTY() {
		return colors{}
	}
	return colors{bold: "\033[1m", dim: "\033[2m", red: "\033[31m", green: "\033[32m", reset: "\033[0m"}
}

func isTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
