package apt

import (
	"apm/cmd/common/reply"
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

// syncAptMutex защищает операции apt-get от дублированного вызова
var syncAptMutex sync.Mutex

type Actions struct{}

func NewActions() *Actions {
	return &Actions{}
}

// PackageChanges Структура, для хранения результатов apt-get -s
type PackageChanges struct {
	ExtraInstalled       []string `json:"extraInstalled"`
	UpgradedPackages     []string `json:"upgradedPackages"`
	NewInstalledPackages []string `json:"newInstalledPackages"`
	RemovedPackages      []string `json:"removedPackages"`

	UpgradedCount     int `json:"upgradedCount"`
	NewInstalledCount int `json:"newInstalledCount"`
	RemovedCount      int `json:"removedCount"`
	NotUpgradedCount  int `json:"notUpgradedCount"`
}

// Package описывает структуру для хранения информации о пакете.
type Package struct {
	Name             string   `json:"name"`
	Section          string   `json:"section"`
	InstalledSize    int      `json:"installedSize"`
	Maintainer       string   `json:"maintainer"`
	Version          string   `json:"version"`
	VersionInstalled string   `json:"versionInstalled"`
	Depends          []string `json:"depends"`
	Size             int      `json:"size"`
	Filename         string   `json:"filename"`
	Description      string   `json:"description"`
	Changelog        string   `json:"lastChangelog"`
	Installed        bool     `json:"installed"`
}

func (a *Actions) Install(ctx context.Context, packageName string) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s apt-get -y install %s", lib.Env.CommandPrefix, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return fmt.Errorf(aptError.GetText())
	}
	if err != nil {
		lib.Log.Errorf("ошибка установки: %s", outputStr)
		return fmt.Errorf("ошибка установки: %v", err)
	}

	return nil
}

func (a *Actions) Remove(ctx context.Context, packageName string) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s apt-get -y remove %s", lib.Env.CommandPrefix, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return fmt.Errorf(aptError.GetText())
	}
	if err != nil {
		lib.Log.Errorf("ошибка установки: %s", outputStr)
		return fmt.Errorf("ошибка удаления пакетов: %v", err)
	}

	return nil
}

func (a *Actions) Check(ctx context.Context, packageName string, aptCommand string) (PackageChanges, string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s apt-get -s %s %s", lib.Env.CommandPrefix, aptCommand, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return PackageChanges{}, "", fmt.Errorf(aptError.GetText())
	}

	if err != nil {
		lib.Log.Errorf("ошибка проверки пакетов: %s", outputStr)
		return PackageChanges{}, "", fmt.Errorf("ошибка проверки пакетов: %v", err)
	}

	packageParse, err := parseAptOutput(outputStr)
	if err != nil {
		return PackageChanges{}, "", fmt.Errorf("ошибка проверки пакета: %v", err)
	}

	return packageParse, outputStr, nil
}

func (a *Actions) Update(ctx context.Context) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	err := aptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("%s apt-cache dumpavail", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ошибка запуска команды: %w", err)
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
				versionValue, errVersion := GetVersionFromAptCache(value)
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
					cleanDep := cleanDependency(dep)
					if cleanDep != "" && !seen[cleanDep] {
						seen[cleanDep] = true
						cleanedDeps = append(cleanedDeps, cleanDep)
					}
				}
				pkg.Depends = cleanedDeps
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

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("слишком большая строка: (over %dMB) - ", maxCapacity/(1024*1024))
		}
		return nil, fmt.Errorf("ошибка сканера: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ошибка выполнения команды: %w", err)
	}
	for i := range packages {
		packages[i].Changelog = extractLastMessage(packages[i].Changelog)
	}

	// Обновляем информацию о том, установлены ли пакеты локально
	packages, err = updateInstalledInfo(packages)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления информации об установленных пакетах: %w", err)
	}

	err = SavePackagesToDB(ctx, packages)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func aptUpdate(ctx context.Context) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s apt-get update", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return fmt.Errorf(aptError.GetText())
	}
	if err != nil {
		return fmt.Errorf("ошибка обновления пакетов: %v, output: %s", err, string(output))
	}

	return nil
}

// updateInstalledInfo обновляет срез пакетов, устанавливая поля Installed и InstalledVersion, если пакет найден в системе.
func updateInstalledInfo(packages []Package) ([]Package, error) {
	installed, err := GetInstalledPackages()
	if err != nil {
		return nil, err
	}

	for i, pkg := range packages {
		if version, found := installed[pkg.Name]; found {
			packages[i].Installed = true
			packages[i].VersionInstalled = version
		}
	}

	return packages, nil
}

// GetInstalledPackages возвращает карту, где ключ – имя пакета, а значение – его установленная версия.
func GetInstalledPackages() (map[string]string, error) {
	command := fmt.Sprintf("%s rpm -qia", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения команды rpm -qia: %w", err)
	}

	installed := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var currentName, currentVersion string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Name") {
			if currentName != "" {
				installed[currentName] = currentVersion
				currentName, currentVersion = "", ""
			}
		}
		if line == "" {
			if currentName != "" {
				installed[currentName] = currentVersion
				currentName, currentVersion = "", ""
			}
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
	}

	if currentName != "" {
		installed[currentName] = currentVersion
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка сканирования вывода rpm: %w", err)
	}

	return installed, nil
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

func cleanDependency(dep string) string {
	re := regexp.MustCompile(`\s*\(.*?\)`)
	return strings.TrimSpace(re.ReplaceAllString(dep, ""))
}

func parseAptOutput(output string) (PackageChanges, error) {
	pc := &PackageChanges{}
	// Разбиваем вывод на строки
	lines := strings.Split(output, "\n")

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
		if matched, _ := regexp.MatchString(`\d+ upgraded, \d+ newly installed, \d+ removed and \d+ not upgraded\.`, line); matched {
			// Пример строки: "3 upgraded, 2 newly installed, 0 removed and 249 not upgraded."
			re := regexp.MustCompile(`(\d+) upgraded, (\d+) newly installed, (\d+) removed and (\d+) not upgraded\.`)
			matches := re.FindStringSubmatch(line)
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

func GetVersionFromAptCache(s string) (string, error) {
	parts := strings.Split(s, ":")
	var candidate string
	if len(parts) > 1 && regexp.MustCompile(`^\d+$`).MatchString(parts[0]) {
		candidate = parts[1]
	} else {
		candidate = parts[0]
	}

	if idx := strings.Index(candidate, "-alt"); idx != -1 {
		numericPart := candidate[:idx]
		if strings.Contains(numericPart, ".") {
			candidate = numericPart
		}
	}

	if candidate == "" {
		return "", fmt.Errorf("version not found")
	}
	return candidate, nil
}
