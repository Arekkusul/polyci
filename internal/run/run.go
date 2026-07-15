// Package run executes a plan's steps, streaming their output, and reports per-step results.
package run

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Arekkusul/polyci/internal/plan"
)

// Status of a single step.
const (
	Pass = "pass"
	Fail = "fail"
	Skip = "skip"
)

// Result is the outcome of one step.
type Result struct {
	Step     plan.Step
	Status   string
	Duration time.Duration
	Err      error
}

// ExecFunc runs a step (streaming to out/errw) and returns its captured stdout.
// It's injectable so tests can run without spawning processes.
type ExecFunc func(step plan.Step, dir string, env []string, out, errw io.Writer) (stdout string, err error)

// Options configures a run.
type Options struct {
	Dir       string
	Env       map[string]string
	KeepGoing bool
	Stdout    io.Writer
	Stderr    io.Writer
	Exec      ExecFunc                     // defaults to DefaultExec
	LookPath  func(string) (string, error) // defaults to exec.LookPath
	Now       func() time.Time             // defaults to time.Now
	OnStart   func(plan.Step)              // called before each step (for live headers)
}

// Run executes steps in order. It stops at the first failure unless KeepGoing is set.
func Run(steps []plan.Step, opts Options) []Result {
	exe := opts.Exec
	if exe == nil {
		exe = DefaultExec
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	env := append(os.Environ(), envPairs(opts.Env)...)

	var results []Result
	for _, s := range steps {
		if opts.OnStart != nil {
			opts.OnStart(s)
		}
		if s.Optional {
			if _, err := lookPath(s.Command[0]); err != nil {
				results = append(results, Result{Step: s, Status: Skip})
				continue
			}
		}
		start := now()
		stdout, err := exe(s, opts.Dir, env, opts.Stdout, opts.Stderr)
		res := Result{Step: s, Status: Pass, Duration: now().Sub(start)}
		if err != nil {
			res.Status, res.Err = Fail, err
		} else if s.FailOnOutput && strings.TrimSpace(stdout) != "" {
			res.Status, res.Err = Fail, fmt.Errorf("command reported: %s", strings.TrimSpace(stdout))
		}
		results = append(results, res)
		if res.Status == Fail && !opts.KeepGoing {
			break
		}
	}
	return results
}

// Failures returns how many results failed.
func Failures(results []Result) int {
	n := 0
	for _, r := range results {
		if r.Status == Fail {
			n++
		}
	}
	return n
}

// DefaultExec runs the step as a subprocess, teeing stdout to out (and capturing it).
func DefaultExec(step plan.Step, dir string, env []string, out, errw io.Writer) (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command(step.Command[0], step.Command[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = io.MultiWriter(out, &buf)
	cmd.Stderr = errw
	err := cmd.Run()
	return buf.String(), err
}

func envPairs(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, k+"="+v)
	}
	return pairs
}
