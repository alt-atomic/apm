package helper

import (
	"apm/lib"
	"bytes"
	"os/exec"
)

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(command string) (string, string, error) {
	lib.Log.Debug(command)
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
