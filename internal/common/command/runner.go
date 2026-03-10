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

	"apm/internal/common/app"
)

const divider = "::::CMD_DIVIDER::::"

// runner реализация Runner
type runner struct {
	commandPrefix string
	verbose       bool
}

// NewRunner создаёт новый Runner.
func NewRunner(commandPrefix string, verbose bool) Runner {
	return &runner{
		commandPrefix: commandPrefix,
		verbose:       verbose,
	}
}

// Option настройка выполнения команды
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
}

// WithEnv добавляет переменные окружения к команде
func WithEnv(env ...string) Option {
	return func(o *options) {
		o.env = append(o.env, env...)
	}
}

// WithPassthrough направляет stdout/stderr напрямую в консоль
func WithPassthrough() Option {
	return func(o *options) {
		o.passthrough = true
	}
}

// WithQuiet подавляет вывод в консоль, даже если runner в verbose-режиме
func WithQuiet() Option {
	return func(o *options) {
		o.quiet = true
	}
}

// WithStdin устанавливает stdin для команды
func WithStdin(r io.Reader) Option {
	return func(o *options) {
		o.stdin = r
	}
}

// WithDir устанавливает рабочую директорию для команды
func WithDir(dir string) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// WithShell выполняет команду через bash -c (для пайпов и спецсимволов)
func WithShell() Option {
	return func(o *options) {
		o.shell = true
	}
}

// WithOutputCommand добавляет вторую команду для захвата дополнительного вывода.
// Результат второй команды записывается в dest
func WithOutputCommand(command string, dest *string) Option {
	return func(o *options) {
		o.outputCommand = command
		o.outputDest = dest
	}
}

// Run выполняет команду с автоматической подстановкой commandPrefix.
func (r *runner) Run(ctx context.Context, args []string, opts ...Option) (string, string, error) {
	var fullArgs []string
	if r.commandPrefix != "" {
		fullArgs = append(fullArgs, strings.Fields(r.commandPrefix)...)
	}
	fullArgs = append(fullArgs, args...)
	return r.execute(ctx, fullArgs, opts...)
}

// Execute выполняет команду с заданными аргументами и опциями.
func (r *runner) execute(ctx context.Context, args []string, opts ...Option) (string, string, error) {
	o := r.applyOptions(opts)

	if o.outputCommand != "" {
		return r.executeDivider(ctx, args, o)
	}

	if o.shell {
		return r.executeShell(ctx, args, o)
	}

	return r.executeCommand(ctx, args, o)
}

// applyOptions применяет опции и учитывает verbose-режим.
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

// executeCommand выполняет прямую команду (без shell).
func (r *runner) executeCommand(ctx context.Context, args []string, o options) (string, string, error) {
	if len(args) == 0 {
		return "", "", ErrEmptyCommand
	}

	app.Log.Debug("run command: ", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	return r.runCmd(cmd, o)
}

// executeShell выполняет команду через sh -c.
func (r *runner) executeShell(ctx context.Context, args []string, o options) (string, string, error) {
	command := strings.Join(args, " ")
	if command == "" {
		return "", "", ErrEmptyCommand
	}

	app.Log.Debug("run shell command: ", command)
	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	return r.runCmd(cmd, o)
}

// executeDivider выполняет основную команду и команду захвата вывода через разделитель.
func (r *runner) executeDivider(ctx context.Context, args []string, o options) (string, string, error) {
	command := strings.Join(args, " ")
	if command == "" {
		return "", "", ErrEmptyCommand
	}

	script := "set -e\n" + command + "\necho '" + divider + "'\n" + o.outputCommand

	app.Log.Debug("run divider command: ", script)
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

// setupCmd настраивает exec.Cmd общими опциями (env, stdin, dir).
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

// runCmd запускает exec.Cmd с настроенными опциями.
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
