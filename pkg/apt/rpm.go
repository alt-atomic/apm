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

package apt

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

// InstalledPackageInfo holds installed package info from rpm
type InstalledPackageInfo struct {
	Name    string
	Version string
	Arch    string
}

// KernelRPMInfo holds kernel package info from rpm
type KernelRPMInfo struct {
	Name      string
	Version   string
	Release   string
	BuildTime string
}

// RpmGetInstalledPackages returns a map of installed packages (name -> version)
func (a *Actions) RpmGetInstalledPackages(ctx context.Context, commandPrefix string, noLock ...bool) (map[string]string, error) {
	var result map[string]string
	skipLock := len(noLock) > 0 && noLock[0]

	err := a.runOperation(OperationOptions{SkipLock: skipLock}, func(_ *lib.System) error {
		command := fmt.Sprintf("%s rpm -qia", commandPrefix)
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Env = []string{"LC_ALL=C"}

		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return fmt.Errorf(T_("Error executing the rpm -qia command: %w"), cmdErr)
		}

		var parseErr error
		result, parseErr = parseRpmQiaOutput(string(output))
		return parseErr
	})

	return result, err
}

// RpmQueryKernelPackages returns installed kernels via rpm
func (a *Actions) RpmQueryKernelPackages(ctx context.Context) ([]KernelRPMInfo, error) {
	var result []KernelRPMInfo

	err := a.runOperation(OperationOptions{}, func(_ *lib.System) error {
		cmd := exec.CommandContext(ctx, "rpm", "-qa", "--queryformat",
			"%{NAME}\t%{VERSION}\t%{RELEASE}\t%{BUILDTIME}\n", "kernel-image-*")
		cmd.Env = []string{"LC_ALL=C"}

		output, cmdErr := cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf(T_("failed to query installed kernels: %s"), cmdErr.Error())
		}

		var parseErr error
		result, parseErr = parseKernelRpmOutput(string(output))
		return parseErr
	})

	return result, err
}

// RpmIsPackageInstalled checks whether the package is installed
func (a *Actions) RpmIsPackageInstalled(packageName string) (bool, error) {
	var installed bool

	err := a.runOperation(OperationOptions{}, func(_ *lib.System) error {
		cmd := exec.Command("rpm", "-q", packageName)
		cmdErr := cmd.Run()
		installed = cmdErr == nil
		return nil // missing package is not an error
	})

	return installed, err
}

// RpmIsAnyPackageInstalled checks whether at least one of the packages is installed
func (a *Actions) RpmIsAnyPackageInstalled(packageNames []string) (bool, error) {
	var installed bool

	err := a.runOperation(OperationOptions{}, func(_ *lib.System) error {
		for _, pkgName := range packageNames {
			cmd := exec.Command("rpm", "-q", pkgName)
			if cmd.Run() == nil {
				installed = true
				return nil
			}
		}
		return nil
	})

	return installed, err
}

// parseRpmQiaOutput parses rpm -qia output into a name -> version map
func parseRpmQiaOutput(output string) (map[string]string, error) {
	installed := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentName, currentVersion, currentArch string

	flushCurrent := func() {
		if currentName == "" {
			return
		}
		name := currentName
		if strings.HasPrefix(name, "i586-") && (currentArch == "i586" || currentArch == "i386") {
			name = strings.TrimPrefix(name, "i586-")
		}

		// If the package already exists, keep the newer version
		if existingVersion, exists := installed[name]; exists {
			if compareVersions(currentVersion, existingVersion) > 0 {
				installed[name] = currentVersion
			}
		} else {
			installed[name] = currentVersion
		}

		currentName, currentVersion, currentArch = "", "", ""
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Name") {
			flushCurrent()
		}
		if line == "" {
			flushCurrent()
			continue
		}

		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentName = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentVersion = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "Architecture") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentArch = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	flushCurrent()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(T_("error scanning RPM output: %w"), err)
	}

	return installed, nil
}

// compareVersions compares two package versions
// Returns: 1 if a > b, -1 if a < b, 0 if equal
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		bVal := 0

		if i < len(aParts) {
			aVal, _ = strconv.Atoi(aParts[i])
		}

		if i < len(bParts) {
			bVal, _ = strconv.Atoi(bParts[i])
		}

		if aVal > bVal {
			return 1
		}
		if aVal < bVal {
			return -1
		}
	}

	return 0
}

// parseKernelRpmOutput parses rpm -qa output for kernels
func parseKernelRpmOutput(output string) ([]KernelRPMInfo, error) {
	var kernels []KernelRPMInfo
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if strings.Contains(name, "debuginfo") {
			continue
		}

		kernels = append(kernels, KernelRPMInfo{
			Name:      name,
			Version:   strings.TrimSpace(parts[1]),
			Release:   strings.TrimSpace(parts[2]),
			BuildTime: strings.TrimSpace(parts[3]),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(T_("error scanning RPM output: %s"), err.Error())
	}

	return kernels, nil
}
