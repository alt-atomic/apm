package _package

import (
	aptParser "apm/internal/common/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/reply"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UpdateKernel выполняет фактическое обновление ядра
func (a *Actions) UpdateKernel(ctx context.Context) []error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.UpdateKernel"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.UpdateKernel"))

	cmdStr := fmt.Sprintf("%s update-kernel -f -y", lib.Env.CommandPrefix)
	errs := aptParser.CommandWithProgress(ctx, cmdStr, aptParser.TypeInstall)

	return errs
}

// CheckUpdateKernel проверка пакетов для обновления ядра
func (a *Actions) CheckUpdateKernel(ctx context.Context) (aptLib.PackageChanges, []error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s update-kernel -n", lib.Env.CommandPrefix))
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	out, runErr := cmd.CombinedOutput()
	rawLines := strings.Split(string(out), "\n")

	cleanLines := filterNoise(rawLines)
	aptErrRaw := aptParser.ErrorLinesAnalyseAll(cleanLines)
	pkgs, parseErr := parseAptOutput(strings.Join(cleanLines, "\n"))

	if len(aptErrRaw) > 0 {
		errs := make([]error, len(aptErrRaw))
		for i, e := range aptErrRaw {
			errs[i] = e
		}
		if parseErr != nil {
			errs = append(errs, fmt.Errorf(lib.T_("Package verification error: %v"), parseErr))
		}
		return pkgs, errs
	}

	if runErr != nil {
		return aptLib.PackageChanges{}, []error{
			fmt.Errorf(lib.T_("update-kernel exited with code %d"), exitCode(runErr)),
		}
	}

	if parseErr != nil {
		return aptLib.PackageChanges{}, []error{
			fmt.Errorf(lib.T_("Package verification error: %v"), parseErr),
		}
	}

	return pkgs, nil
}

// exitCode достаёт код возврата из *exec.ExitError.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// filterNoise убирает строки, которые не относятся к выводу apt-get.
func filterNoise(lines []string) []string {
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		switch {
		case l == "":
			continue
		case strings.HasPrefix(l, "ATTENTION:"):
			continue
		case strings.HasPrefix(l, "Running kernel:"),
			strings.HasPrefix(l, "Checking for available"),
			strings.HasPrefix(l, "Kernel "),
			strings.HasPrefix(l, "Latest available kernel"),
			strings.HasPrefix(l, "List of available kernels"):
			continue
		default:
			cleaned = append(cleaned, l)
		}
	}
	return cleaned
}

func parseAptOutput(output string) (aptLib.PackageChanges, error) {
	pc := &aptLib.PackageChanges{}
	lines := strings.Split(output, "\n")

	statsRegex := regexp.MustCompile(`(\d+) upgraded, (\d+) newly installed, (\d+) removed and (\d+) not upgraded\.`)

	var currentSection string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Определяем заголовки секций
		if strings.HasPrefix(line, "The following extra packages will be installed:") {
			currentSection = "extra_installed"
			continue
		}
		if strings.HasPrefix(line, "The following packages will be upgraded") {
			currentSection = "upgraded"
			continue
		}
		if strings.HasPrefix(line, "The following NEW packages will be installed:") {
			currentSection = "new_installed"
			continue
		}
		if strings.HasPrefix(line, "The following packages will be REMOVED:") {
			currentSection = "removed"
			continue
		}

		// Если строка содержит статистику, то обрабатываем отдельно
		if statsRegex.MatchString(line) {
			matches := statsRegex.FindStringSubmatch(line)
			if len(matches) == 5 {
				if count, err := strconv.Atoi(matches[1]); err == nil {
					pc.UpgradedCount = count
				}
				if count, err := strconv.Atoi(matches[2]); err == nil {
					pc.NewInstalledCount = count
				}
				if count, err := strconv.Atoi(matches[3]); err == nil {
					pc.RemovedCount = count
				}
				if count, err := strconv.Atoi(matches[4]); err == nil {
					pc.NotUpgradedCount = count
				}
			}
			currentSection = ""
			continue
		}

		if strings.HasSuffix(line, "...") {
			continue
		}
		switch currentSection {
		case "extra_installed":
			pkgs := strings.Fields(line)
			pc.ExtraInstalled = append(pc.ExtraInstalled, pkgs...)
		case "upgraded":
			pkgs := strings.Fields(line)
			pc.UpgradedPackages = append(pc.UpgradedPackages, pkgs...)
		case "new_installed":
			pkgs := strings.Fields(line)
			pc.NewInstalledPackages = append(pc.NewInstalledPackages, pkgs...)
		case "removed":
			pkgs := strings.Fields(line)
			pc.RemovedPackages = append(pc.RemovedPackages, pkgs...)
		}
	}

	return *pc, nil
}
