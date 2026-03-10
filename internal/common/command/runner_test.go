// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package command

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunCapturesStdout(t *testing.T) {
	r := NewRunner("", false)

	stdout, stderr, err := r.Run(context.Background(), []string{"echo", "hello world"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "hello world" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "hello world")
	}
	if stderr != "" {
		t.Errorf("Run() stderr = %q, want empty", stderr)
	}
}

func TestRunWithPrefix(t *testing.T) {
	r := NewRunner("env", false)

	stdout, _, err := r.Run(context.Background(), []string{"echo", "prefixed"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "prefixed" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "prefixed")
	}
}

func TestRunWithCompoundPrefix(t *testing.T) {
	r := NewRunner("env LC_ALL=C", false)

	stdout, _, err := r.Run(context.Background(), []string{"echo", "compound"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "compound" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "compound")
	}
}

func TestRunEmptyArgs(t *testing.T) {
	r := NewRunner("", false)

	_, _, err := r.Run(context.Background(), []string{})
	if !errors.Is(err, ErrEmptyCommand) {
		t.Errorf("Run() error = %v, want ErrEmptyCommand", err)
	}
}

func TestRunFailedCommand(t *testing.T) {
	r := NewRunner("", false)

	_, stderr, err := r.Run(context.Background(), []string{"ls", "/nonexistent-path-12345"})
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if stderr == "" {
		t.Error("Run() expected non-empty stderr")
	}
}

func TestRunContextCancelled(t *testing.T) {
	r := NewRunner("", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := r.Run(ctx, []string{"sleep", "10"})
	if err == nil {
		t.Fatal("Run() expected error with cancelled context")
	}
}

func TestRunWithEnv(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"sh", "-c", "echo $TEST_APM_VAR"},
		WithEnv("TEST_APM_VAR=test_value"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "test_value" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "test_value")
	}
}

func TestRunWithMultipleEnv(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"sh", "-c", "echo $VAR_A $VAR_B"},
		WithEnv("VAR_A=hello", "VAR_B=world"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "hello world" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "hello world")
	}
}

func TestRunWithShell(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"echo hello | tr a-z A-Z"},
		WithShell(),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "HELLO" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "HELLO")
	}
}

func TestRunWithShellAndEnv(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"echo $SHELL_VAR"},
		WithShell(),
		WithEnv("SHELL_VAR=combined"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "combined" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "combined")
	}
}

func TestRunWithShellEmpty(t *testing.T) {
	r := NewRunner("", false)

	_, _, err := r.Run(context.Background(), []string{}, WithShell())
	if !errors.Is(err, ErrEmptyCommand) {
		t.Errorf("Run() error = %v, want ErrEmptyCommand", err)
	}
}

func TestRunWithDir(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"pwd"},
		WithDir("/tmp"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "/tmp" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "/tmp")
	}
}

func TestRunWithDirShell(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"pwd"},
		WithShell(),
		WithDir("/tmp"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "/tmp" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "/tmp")
	}
}

func TestRunWithStdin(t *testing.T) {
	r := NewRunner("", false)

	input := strings.NewReader("stdin data")
	stdout, _, err := r.Run(context.Background(),
		[]string{"cat"},
		WithStdin(input),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout != "stdin data" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "stdin data")
	}
}

func TestRunWithPassthroughCapturesOutput(t *testing.T) {
	r := NewRunner("", false)

	stdout, stderr, err := r.Run(context.Background(),
		[]string{"echo", "passthrough"},
		WithPassthrough(),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "passthrough" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "passthrough")
	}
	if stderr != "" {
		t.Errorf("Run() stderr = %q, want empty", stderr)
	}
}

func TestVerboseAutoPassthrough(t *testing.T) {
	r := NewRunner("", true)

	stdout, _, err := r.Run(context.Background(), []string{"echo", "verbose"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "verbose" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "verbose")
	}
}

func TestVerboseWithExplicitPassthrough(t *testing.T) {
	r := NewRunner("", true)

	stdout, _, err := r.Run(context.Background(),
		[]string{"echo", "both"},
		WithPassthrough(),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "both" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "both")
	}
}

func TestQuietSuppressesVerbose(t *testing.T) {
	r := NewRunner("", true)

	stdout, _, err := r.Run(context.Background(),
		[]string{"echo", "quiet"},
		WithQuiet(),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "quiet" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "quiet")
	}
}

func TestQuietOverridesPassthrough(t *testing.T) {
	r := NewRunner("", false)

	stdout, _, err := r.Run(context.Background(),
		[]string{"echo", "silent"},
		WithPassthrough(),
		WithQuiet(),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "silent" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "silent")
	}
}

func TestRunWithOutputCommand(t *testing.T) {
	r := NewRunner("", false)

	var output string
	stdout, _, err := r.Run(context.Background(),
		[]string{"echo main_output"},
		WithOutputCommand("echo second_output", &output),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "main_output" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "main_output")
	}
	if strings.TrimSpace(output) != "second_output" {
		t.Errorf("output = %q, want %q", output, "second_output")
	}
}

func TestRunWithOutputCommandMultiline(t *testing.T) {
	r := NewRunner("", false)

	var output string
	stdout, _, err := r.Run(context.Background(),
		[]string{"echo line1; echo line2"},
		WithOutputCommand("echo result1; echo result2", &output),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "line1\nline2" {
		t.Errorf("Run() stdout = %q, want %q", strings.TrimSpace(stdout), "line1\nline2")
	}
	if strings.TrimSpace(output) != "result1\nresult2" {
		t.Errorf("output = %q, want %q", strings.TrimSpace(output), "result1\nresult2")
	}
}

func TestRunWithOutputCommandEmptyArgs(t *testing.T) {
	r := NewRunner("", false)

	var output string
	_, _, err := r.Run(context.Background(), []string{}, WithOutputCommand("echo x", &output))
	if !errors.Is(err, ErrEmptyCommand) {
		t.Errorf("Run() error = %v, want ErrEmptyCommand", err)
	}
}

func TestRunWithOutputCommandFailedCommand(t *testing.T) {
	r := NewRunner("", false)

	var output string
	_, _, err := r.Run(context.Background(),
		[]string{"echo fail_msg >&2; exit 1"},
		WithOutputCommand("echo after", &output),
	)
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
}

func TestRunWithOutputCommandAndEnv(t *testing.T) {
	r := NewRunner("", false)

	var output string
	stdout, _, err := r.Run(context.Background(),
		[]string{"echo $DIV_VAR"},
		WithOutputCommand("echo $DIV_VAR", &output),
		WithEnv("DIV_VAR=env_test"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "env_test" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "env_test")
	}
	if strings.TrimSpace(output) != "env_test" {
		t.Errorf("output = %q, want %q", output, "env_test")
	}
}

func TestRunWithOutputCommandAndDir(t *testing.T) {
	r := NewRunner("", false)

	var output string
	stdout, _, err := r.Run(context.Background(),
		[]string{"pwd"},
		WithOutputCommand("pwd", &output),
		WithDir("/tmp"),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "/tmp" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "/tmp")
	}
	if strings.TrimSpace(output) != "/tmp" {
		t.Errorf("output = %q, want %q", output, "/tmp")
	}
}

func TestRunWithOutputCommandStderrAlwaysReturned(t *testing.T) {
	r := NewRunner("", false)

	var output string
	stdout, stderr, err := r.Run(context.Background(),
		[]string{"echo main_out; echo err_msg >&2"},
		WithOutputCommand("echo captured", &output),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout) != "main_out" {
		t.Errorf("Run() stdout = %q, want %q", stdout, "main_out")
	}
	if strings.TrimSpace(stderr) != "err_msg" {
		t.Errorf("Run() stderr = %q, want %q", stderr, "err_msg")
	}
	if strings.TrimSpace(output) != "captured" {
		t.Errorf("output = %q, want %q", output, "captured")
	}
}

func TestDividerWriterSingleWrite(t *testing.T) {
	var main, output strings.Builder
	dw := newDividerWriter(&main, &output, "===")

	_, err := dw.Write([]byte("before===after"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_ = dw.flush()

	if main.String() != "before" {
		t.Errorf("main = %q, want %q", main.String(), "before")
	}
	if output.String() != "after" {
		t.Errorf("output = %q, want %q", output.String(), "after")
	}
}

func TestDividerWriterSplitAcrossWrites(t *testing.T) {
	var main, output strings.Builder
	dw := newDividerWriter(&main, &output, "===")

	_, _ = dw.Write([]byte("before="))
	_, _ = dw.Write([]byte("==after"))
	_ = dw.flush()

	if main.String() != "before" {
		t.Errorf("main = %q, want %q", main.String(), "before")
	}
	if output.String() != "after" {
		t.Errorf("output = %q, want %q", output.String(), "after")
	}
}

func TestDividerWriterNoDivider(t *testing.T) {
	var main, output strings.Builder
	dw := newDividerWriter(&main, &output, "===")

	_, _ = dw.Write([]byte("no divider here"))
	_ = dw.flush()

	if main.String() != "no divider here" {
		t.Errorf("main = %q, want %q", main.String(), "no divider here")
	}
	if output.String() != "" {
		t.Errorf("output = %q, want empty", output.String())
	}
}

func TestDividerWriterNewlineAfterDivider(t *testing.T) {
	var main, output strings.Builder
	dw := newDividerWriter(&main, &output, "===")

	_, _ = dw.Write([]byte("before===\nafter"))
	_ = dw.flush()

	if main.String() != "before" {
		t.Errorf("main = %q, want %q", main.String(), "before")
	}
	if output.String() != "after" {
		t.Errorf("output = %q, want %q", output.String(), "after")
	}
}

func TestDividerWriterAfterDividerAllGoesToOutput(t *testing.T) {
	var main, output strings.Builder
	dw := newDividerWriter(&main, &output, "===")

	_, _ = dw.Write([]byte("before===\n"))
	_, _ = dw.Write([]byte("line1\n"))
	_, _ = dw.Write([]byte("line2\n"))
	_ = dw.flush()

	if main.String() != "before" {
		t.Errorf("main = %q, want %q", main.String(), "before")
	}
	if output.String() != "line1\nline2\n" {
		t.Errorf("output = %q, want %q", output.String(), "line1\nline2\n")
	}
}
