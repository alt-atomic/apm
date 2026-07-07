package aptrepo

import (
	"context"

	"altlinux.space/alt-atomic/apm/pkg/command"
)

// commandRunner runs external commands.
type commandRunner interface {
	Run(ctx context.Context, args []string, opts ...command.Option) (string, string, error)
}

// HasPackageFunc reports whether the named package is installed.
type HasPackageFunc func(ctx context.Context, name string) bool
