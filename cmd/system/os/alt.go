package os

import (
	"apm/lib"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PackageInfo описывает основные сведения о пакете.
type PackageInfo struct {
	Name        string
	Version     string
	Size        string
	URL         string
	Summary     string
	InstallDate string
}

// GetPackageInfo выполняет команду `rpm -qi <pkg>`, парсит результат и возвращает PackageInfo.
// Если команда завершается с ошибкой, stderr включается в сообщение об ошибке.
func GetPackageInfo(pkg string) (PackageInfo, error) {
	command := fmt.Sprintf("%s rpm -qi %s", lib.Env.CommandPrefix, pkg)
	stdout, stderr, err := RunCommand(command)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("ошибка получения информации о пакете: %s%s", stderr, stdout)
	}

	lines := strings.Split(stdout, "\n")
	info := PackageInfo{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		fieldName := strings.TrimSpace(parts[0])
		fieldValue := strings.TrimSpace(parts[1])

		switch fieldName {
		case "Name":
			info.Name = fieldValue
		case "Version":
			info.Version = fieldValue
		case "Install Date":
			info.InstallDate = fieldValue
		case "Size":
			sizeInt, err := strconv.ParseInt(fieldValue, 10, 64)
			if err != nil {
				info.Size = fieldValue
			} else {
				mb := float64(sizeInt) / (1024 * 1024)
				info.Size = fmt.Sprintf("%.2f MB", mb)
			}
		case "URL":
			info.URL = fieldValue
		case "Summary":
			info.Summary = fieldValue
		}
	}

	return info, nil
}

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(command string) (string, string, error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
