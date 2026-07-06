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
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

const divider = "::::CMD_DIVIDER::::"

// runner implements Runner
type runner struct {
	commandPrefix string
	verbose       bool
}

// NewRunner creates a new Runner.
func NewRunner(commandPrefix string, verbose bool) Runner {
	return &runner{
		commandPrefix: commandPrefix,
		verbose:       verbose,
	}
}

// Option configures command execution
type Option func(*options)

type options struct {
	env           []string
	passthrough   bool
	quiet         bool
	stdin         io.Reader
	shell         bool
	dir           string
	outputCommand string
	outputDest    *string
	pty           bool
	ptyRows       uint16
	ptyCols       uint16
	streamHandler func(io.Reader)
}

// WithEnv adds environment variables to the command
func WithEnv(env ...string) Option {
	return func(o *options) {
		o.env = append(o.env, env...)
	}
}

// WithPassthrough routes stdout/stderr directly to the console
func WithPassthrough() Option {
	return func(o *options) {
		o.passthrough = true
	}
}

// WithQuiet suppresses console output even if the runner is verbose
func WithQuiet() Option {
	return func(o *options) {
		o.quiet = true
	}
}

// WithStdin sets stdin for the command
func WithStdin(r io.Reader) Option {
	return func(o *options) {
		o.stdin = r
	}
}

// WithDir sets the working directory for the command
func WithDir(dir string) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// WithShell runs the command via bash -c (for pipes and special characters)
func WithShell() Option {
	return func(o *options) {
		o.shell = true
	}
}

// WithOutputCommand adds a second command to capture extra output.
// The second command's result is written to dest.
func WithOutputCommand(command string, dest *string) Option {
	return func(o *options) {
		o.outputCommand = command
		o.outputDest = dest
	}
}

// WithPTY runs the command in a pseudo-tty with the given window size.
// Used for commands that show interactive progress.
func WithPTY(rows, cols uint16) Option {
	return func(o *options) {
		o.pty = true
		o.ptyRows = rows
		o.ptyCols = cols
	}
}

// WithStreamHandler registers a stdout stream handler.
func WithStreamHandler(h func(io.Reader)) Option {
	return func(o *options) {
		o.streamHandler = h
	}
}

// Run executes a command with commandPrefix applied.
func (r *runner) Run(ctx context.Context, args []string, opts ...Option) (string, string, error) {
	var fullArgs []string
	if r.commandPrefix != "" {
		fullArgs = append(fullArgs, strings.Fields(r.commandPrefix)...)
	}
	fullArgs = append(fullArgs, args...)
	return r.execute(ctx, fullArgs, opts...)
}

// Execute executes a command with the given arguments and options.
func (r *runner) execute(ctx context.Context, args []string, opts ...Option) (string, string, error) {
	o := r.applyOptions(opts)

	if o.pty {
		return r.executePTY(ctx, args, o)
	}

	if o.outputCommand != "" {
		return r.executeDivider(ctx, args, o)
	}

	if o.shell {
		return r.executeShell(ctx, args, o)
	}

	return r.executeCommand(ctx, args, o)
}

// applyOptions applies options and honors verbose mode.
func (r *runner) applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if o.quiet {
		o.passthrough = false
	} else if r.verbose && !o.passthrough {
		o.passthrough = true
	}

	return o
}

// executeCommand executes a direct command (no shell).
func (r *runner) executeCommand(ctx context.Context, args []string, o options) (string, string, error) {
	if len(args) == 0 {
		return "", "", ErrEmptyCommand
	}

	logger.Debug("run command: ", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	return r.runCmd(cmd, o)
}

// executeShell executes a command via sh -c.
func (r *runner) executeShell(ctx context.Context, args []string, o options) (string, string, error) {
	command := strings.Join(args, " ")
	if command == "" {
		return "", "", ErrEmptyCommand
	}

	logger.Debug("run shell command: ", command)
	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	return r.runCmd(cmd, o)
}

// executeDivider executes the main command and the output-capture command via a divider.
func (r *runner) executeDivider(ctx context.Context, args []string, o options) (string, string, error) {
	command := strings.Join(args, " ")
	if command == "" {
		return "", "", ErrEmptyCommand
	}

	script := "set -e\n" + command + "\necho '" + divider + "'\n" + o.outputCommand

	logger.Debug("run divider command: ", script)
	cmd := exec.CommandContext(ctx, "bash", "-c", script)

	r.setupCmd(cmd, o)

	var mainBuf, outputBuf, stderr bytes.Buffer

	var mainWriter io.Writer = &mainBuf
	if o.passthrough {
		mainWriter = io.MultiWriter(os.Stdout, &mainBuf)
	}

	dw := newDividerWriter(mainWriter, &outputBuf, divider)
	cmd.Stdout = dw

	if o.passthrough {
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		cmd.Stderr = &stderr
	}

	err := cmd.Run()

	if flushErr := dw.flush(); flushErr != nil && err == nil {
		err = flushErr
	}

	if o.outputDest != nil {
		*o.outputDest = outputBuf.String()
	}

	if err != nil {
		return mainBuf.String(), stderr.String(), err
	}

	return mainBuf.String(), stderr.String(), nil
}

// setupCmd configures exec.Cmd with common options (env, stdin, dir).
func (r *runner) setupCmd(cmd *exec.Cmd, o options) {
	if len(o.env) > 0 {
		cmd.Env = append(os.Environ(), o.env...)
	}

	if o.stdin != nil {
		cmd.Stdin = o.stdin
	}

	if o.dir != "" {
		cmd.Dir = o.dir
	}
}

// runCmd runs exec.Cmd with the configured options.
func (r *runner) runCmd(cmd *exec.Cmd, o options) (string, string, error) {
	r.setupCmd(cmd, o)

	var stdout, stderr bytes.Buffer

	if o.passthrough {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
