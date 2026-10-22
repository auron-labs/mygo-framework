package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/internal/generator"
	"strings"
	"testing"
)

func TestMyGoCommandsGenerateOnceAndDelegateArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		check bool
	}{
		{name: "dev"},
		{name: "build"},
		{name: "doctor", check: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			generateCalls := 0
			runCalls := 0
			generate := func(ctx context.Context, config generator.GenerateConfig) (generator.GenerateResult, error) {
				generateCalls++
				if ctx != t.Context() || config.Dir == "" || config.Check != test.check {
					t.Fatalf("generate config = %+v, context matches = %t", config, ctx == t.Context())
				}
				return generator.GenerateResult{}, nil
			}
			run := func(ctx context.Context, directory string, _ io.Reader, stdout, stderr io.Writer, args ...string) error {
				runCalls++
				if ctx != t.Context() || directory == "" {
					t.Fatalf("runner directory = %q, context matches = %t", directory, ctx == t.Context())
				}
				if strings.Join(args, " ") != test.name+" --target linux" {
					t.Fatalf("runner args = %v", args)
				}
				_, _ = fmt.Fprint(stdout, "delegated stdout\n")
				_, _ = fmt.Fprint(stderr, "delegated stderr\n")
				return nil
			}

			command := newMyGoCommand(test.name, test.check, generate, run)
			root := New("test", command)
			var stderr strings.Builder
			root.SetErr(&stderr)
			root.SetArgs([]string{test.name, "--target", "linux"})
			var stdout strings.Builder
			root.SetOut(&stdout)
			if err := root.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if generateCalls != 1 || runCalls != 1 {
				t.Fatalf("calls = generate %d, run %d", generateCalls, runCalls)
			}
			if stdout.String() != "delegated stdout\n" || stderr.String() != "delegated stderr\n" {
				t.Fatalf("streams = %q, %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestMyGoCommandStopsBeforeDelegationWhenGenerationFails(t *testing.T) {
	t.Parallel()
	want := errors.New("stale generated output")
	generateCalls := 0
	runCalls := 0
	command := newMyGoCommand("doctor", true,
		func(context.Context, generator.GenerateConfig) (generator.GenerateResult, error) {
			generateCalls++
			return generator.GenerateResult{}, want
		},
		func(context.Context, string, io.Reader, io.Writer, io.Writer, ...string) error {
			runCalls++
			return nil
		},
	)

	_, err := execute(context.Background(), New("test", command), "doctor")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if generateCalls != 1 || runCalls != 0 {
		t.Fatalf("calls = generate %d, run %d", generateCalls, runCalls)
	}
}

func TestMyGoCommandReturnsDelegationFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("tool failed")
	command := newMyGoCommand("build", false,
		func(context.Context, generator.GenerateConfig) (generator.GenerateResult, error) {
			return generator.GenerateResult{}, nil
		},
		func(context.Context, string, io.Reader, io.Writer, io.Writer, ...string) error {
			return want
		},
	)

	_, err := execute(context.Background(), New("test", command), "build")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
