package command

import "context"

// Runner is the interface for executing commands
type Runner interface {
	// Run executes a command with commandPrefix applied and verbose mode honored
	// Returns stdout, stderr, error
	Run(ctx context.Context, args []string, opts ...Option) (string, string, error)
}
