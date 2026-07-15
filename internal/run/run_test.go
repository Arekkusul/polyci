package run

import (
	"errors"
	"io"
	"testing"

	"github.com/Arekkusul/polyci/internal/plan"
)

func steps(names ...string) []plan.Step {
	var s []plan.Step
	for _, n := range names {
		s = append(s, plan.Step{Name: n, Command: []string{n}})
	}
	return s
}

func TestRunAllPass(t *testing.T) {
	results := Run(steps("a", "b", "c"), Options{
		Exec: func(plan.Step, string, []string, io.Writer, io.Writer) (string, error) { return "", nil },
	})
	if len(results) != 3 || Failures(results) != 0 {
		t.Fatalf("expected 3 passes, got %+v", results)
	}
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	var ran []string
	results := Run(steps("a", "b", "c"), Options{
		Exec: func(s plan.Step, _ string, _ []string, _, _ io.Writer) (string, error) {
			ran = append(ran, s.Name)
			if s.Name == "b" {
				return "", errors.New("boom")
			}
			return "", nil
		},
	})
	if len(ran) != 2 || ran[1] != "b" {
		t.Errorf("should stop after b, ran %v", ran)
	}
	if Failures(results) != 1 {
		t.Errorf("expected 1 failure, got %d", Failures(results))
	}
}

func TestRunKeepGoingRunsAll(t *testing.T) {
	var ran []string
	Run(steps("a", "b", "c"), Options{
		KeepGoing: true,
		Exec: func(s plan.Step, _ string, _ []string, _, _ io.Writer) (string, error) {
			ran = append(ran, s.Name)
			return "", errors.New("boom")
		},
	})
	if len(ran) != 3 {
		t.Errorf("keep-going should run all, ran %v", ran)
	}
}

func TestFailOnOutputFailsEvenWithZeroExit(t *testing.T) {
	s := []plan.Step{{Name: "gofmt", Command: []string{"gofmt"}, FailOnOutput: true}}
	results := Run(s, Options{
		Exec: func(plan.Step, string, []string, io.Writer, io.Writer) (string, error) {
			return "main.go\n", nil // exit 0 but printed a file -> unformatted
		},
	})
	if Failures(results) != 1 {
		t.Errorf("gofmt output should fail the step, got %+v", results)
	}
}

func TestFailOnOutputPassesWhenSilent(t *testing.T) {
	s := []plan.Step{{Name: "gofmt", Command: []string{"gofmt"}, FailOnOutput: true}}
	results := Run(s, Options{
		Exec: func(plan.Step, string, []string, io.Writer, io.Writer) (string, error) {
			return "  \n", nil // only whitespace -> clean
		},
	})
	if Failures(results) != 0 {
		t.Errorf("whitespace-only output should pass, got %+v", results)
	}
}

func TestOptionalStepSkippedWhenToolMissing(t *testing.T) {
	s := []plan.Step{{Name: "ruff", Command: []string{"ruff"}, Optional: true}}
	called := false
	results := Run(s, Options{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
		Exec: func(plan.Step, string, []string, io.Writer, io.Writer) (string, error) {
			called = true
			return "", nil
		},
	})
	if called {
		t.Error("exec should not run for a missing optional tool")
	}
	if len(results) != 1 || results[0].Status != Skip {
		t.Errorf("expected skip, got %+v", results)
	}
}
