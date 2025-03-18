package helper

import (
	"apm/lib"
	"bytes"
	"context"
	"os/exec"
)

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(ctx context.Context, command string) (string, string, error) {
	lib.Log.Debug("run command: ", command)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
