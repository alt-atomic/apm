package _package

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// UpdateKernel выполняет фактическое обновление ядра
func (a *Actions) UpdateKernel(ctx context.Context) []error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.UpdateKernel"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.UpdateKernel"))

	cmdStr := fmt.Sprintf("%s update-kernel -f -y", lib.Env.CommandPrefix)
	errs := a.commandWithProgress(ctx, cmdStr, typeInstall)

	return errs
}

// CheckUpdateKernel проверка пакетов для обновления ядра
func (a *Actions) CheckUpdateKernel(ctx context.Context) (PackageChanges, []error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s update-kernel -n", lib.Env.CommandPrefix))
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	out, runErr := cmd.CombinedOutput()
	rawLines := strings.Split(string(out), "\n")

	cleanLines := filterNoise(rawLines)
	aptErrRaw := ErrorLinesAnalyseAll(cleanLines)
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
		return PackageChanges{}, []error{
			fmt.Errorf(lib.T_("update-kernel exited with code %d"), exitCode(runErr)),
		}
	}

	if parseErr != nil {
		return PackageChanges{}, []error{
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
