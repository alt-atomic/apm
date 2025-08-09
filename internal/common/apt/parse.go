package apt

import (
	"apm/internal/common/appstream"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Package struct {
	Name             string               `json:"name"`
	Section          string               `json:"section"`
	InstalledSize    int                  `json:"installedSize"`
	Maintainer       string               `json:"maintainer"`
	Version          string               `json:"version"`
	VersionInstalled string               `json:"versionInstalled"`
	Depends          []string             `json:"depends"`
	Provides         []string             `json:"provides"`
	Size             int                  `json:"size"`
	Filename         string               `json:"filename"`
	Description      string               `json:"description"`
	AppStream        *appstream.Component `json:"appStream"`
	Changelog        string               `json:"lastChangelog"`
	Installed        bool                 `json:"installed"`
	TypePackage      int                  `json:"typePackage"`
}

var syncAptMutex sync.Mutex

func Update(ctx context.Context) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Update"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Update"))

	err := aptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("%s apt-cache dumpavail", lib.Env.CommandPrefix)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(lib.T_("Error opening stdout pipe: %w"), err)
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf(lib.T_("Error executing command: %w"), err)
	}

	const maxCapacity = 1024 * 1024 * 350 // 350MB
	buf := make([]byte, maxCapacity)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(buf, maxCapacity)

	var packages []Package
	var pkg Package
	var currentKey string

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			if pkg.Name != "" {
				packages = append(packages, pkg)
				pkg = Package{}
				currentKey = ""
			}
			continue
		}

		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			currentKey = key

			switch key {
			case "Package":
				pkg.Name = value
			case "Section":
				pkg.Section = value
			case "Installed Size":
				sizeValue, err := strconv.Atoi(value)
				if err != nil {
					sizeValue = 0
				}

				pkg.InstalledSize = sizeValue
			case "Maintainer":
				pkg.Maintainer = value
			case "Version":
				versionValue, errVersion := helper.GetVersionFromAptCache(value)
				if errVersion != nil {
					pkg.Version = value
				} else {
					pkg.Version = versionValue
				}
			case "Depends":
				depList := strings.Split(value, ",")
				seen := make(map[string]bool)
				var cleanedDeps []string
				for _, dep := range depList {
					cleanDep := CleanDependency(dep)
					if cleanDep != "" && !seen[cleanDep] {
						seen[cleanDep] = true
						cleanedDeps = append(cleanedDeps, cleanDep)
					}
				}
				pkg.Depends = cleanedDeps
			case "Provides":
				provList := strings.Split(value, ",")
				seen := make(map[string]bool)
				var cleanedProviders []string
				for _, prov := range provList {
					cleanProv := CleanDependency(prov)
					if cleanProv != "" && !seen[cleanProv] {
						seen[cleanProv] = true
						cleanedProviders = append(cleanedProviders, cleanProv)
					}
				}
				pkg.Provides = cleanedProviders
			case "Size":
				sizeValue, err := strconv.Atoi(value)
				if err != nil {
					sizeValue = 0
				}

				pkg.Size = sizeValue
			case "Filename":
				pkg.Filename = value
			case "Description":
				pkg.Description = value
			case "Changelog":
				pkg.Changelog = value
			default:
			}
		} else {
			switch currentKey {
			case "Description":
				pkg.Description += "\n" + line
			case "Changelog":
				pkg.Changelog += "\n" + line
			default:
			}
		}
	}

	if pkg.Name != "" {
		packages = append(packages, pkg)
	}

	if err = scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf(lib.T_("String too large: (over %dMB) - "), maxCapacity/(1024*1024))
		}
		return nil, fmt.Errorf(lib.T_("Scanner error: %w"), err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf(lib.T_("Command execution error: %w"), err)
	}

	// добавляем Changelog
	for i := range packages {
		packages[i].Changelog = extractLastMessage(packages[i].Changelog)
	}

	return packages, nil
}

func aptUpdate(ctx context.Context) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.AptUpdate"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.AptUpdate"))

	command := fmt.Sprintf("%s apt-get update", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return errors.New(aptError.Error())
	}
	if err != nil {
		return fmt.Errorf(lib.T_("Error updating packages: %v, output: %s"), err, string(output))
	}

	return nil
}

func extractLastMessage(changelog string) string {
	lines := strings.Split(changelog, "\n")
	var result []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "*") {
			if !found {
				result = append(result, trimmed)
				found = true
			} else {
				break
			}
		} else if found {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

func CleanDependency(dep string) string {
	re := regexp.MustCompile(`\s*\(.*?\)`)
	return strings.TrimSpace(re.ReplaceAllString(dep, ""))
}
