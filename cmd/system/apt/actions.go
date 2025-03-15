package apt

import (
	"apm/cmd/common/helper"
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

type Actions struct {
	serviceAptDatabase *PackageDBService
}

func NewActions(serviceAptDatabase *PackageDBService) *Actions {
	return &Actions{
		serviceAptDatabase: serviceAptDatabase,
	}
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
	Provides         []string `json:"provides"`
	Size             int      `json:"size"`
	Filename         string   `json:"filename"`
	Description      string   `json:"description"`
	Changelog        string   `json:"lastChangelog"`
	Installed        bool     `json:"installed"`
}

func (a *Actions) Install(ctx context.Context, packageName string) []error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Install"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Install"))

	command := fmt.Sprintf("%s apt-get -y install %s", lib.Env.CommandPrefix, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptErrors := ErrorLinesAnalyseAll(lines)

	if len(aptErrors) > 0 {
		var errorsSlice []error
		for _, e := range aptErrors {
			errorsSlice = append(errorsSlice, e)
		}
		return errorsSlice
	}

	if err != nil {
		lib.Log.Errorf("ошибка установки: %s", outputStr)
		return []error{fmt.Errorf("ошибка установки: %v", err)}
	}

	return nil
}

func (a *Actions) Remove(ctx context.Context, packageName string) []error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Remove"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Remove"))

	command := fmt.Sprintf("%s apt-get -y remove %s", lib.Env.CommandPrefix, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptErrors := ErrorLinesAnalyseAll(lines)

	if len(aptErrors) > 0 {
		var errorsSlice []error
		for _, e := range aptErrors {
			errorsSlice = append(errorsSlice, e)
		}
		return errorsSlice
	}

	if err != nil {
		lib.Log.Errorf("ошибка удаления: %s", outputStr)
		return []error{fmt.Errorf("ошибка удаления: %v", err)}
	}

	return nil
}

func (a *Actions) Check(ctx context.Context, packageName string, aptCommand string) (PackageChanges, []error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	command := fmt.Sprintf("%s apt-get -s %s %s", lib.Env.CommandPrefix, aptCommand, packageName)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptErrors := ErrorLinesAnalyseAll(lines)

	var packageParse PackageChanges
	if len(aptErrors) > 0 {
		var errorsSlice []error
		for _, e := range aptErrors {
			errorsSlice = append(errorsSlice, e)
		}

		packageParse, err = parseAptOutput(outputStr)
		if err != nil {
			return PackageChanges{}, []error{fmt.Errorf("ошибка проверки пакета: %v", err)}
		}
		return packageParse, errorsSlice
	}

	if err != nil {
		lib.Log.Errorf("ошибка проверки пакетов: %s", outputStr)
		return PackageChanges{}, []error{fmt.Errorf("ошибка проверки пакетов: %v", err)}
	}

	packageParse, err = parseAptOutput(outputStr)
	if err != nil {
		return PackageChanges{}, []error{fmt.Errorf("ошибка проверки пакета: %v", err)}
	}

	return packageParse, nil
}

func (a *Actions) Update(ctx context.Context) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Update"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Update"))

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
					cleanDep := cleanDependency(dep)
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
					cleanProv := cleanDependency(prov)
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
			return nil, fmt.Errorf("слишком большая строка: (over %dMB) - ", maxCapacity/(1024*1024))
		}
		return nil, fmt.Errorf("ошибка сканера: %w", err)
	}
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ошибка выполнения команды: %w", err)
	}
	for i := range packages {
		packages[i].Changelog = extractLastMessage(packages[i].Changelog)
	}

	// Обновляем информацию о том, установлены ли пакеты локально
	packages, err = a.updateInstalledInfo(packages)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления информации об установленных пакетах: %w", err)
	}

	err = a.serviceAptDatabase.SavePackagesToDB(ctx, packages)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

// CleanPackageName очищаем странный суффикс в ответе apt
func (a *Actions) CleanPackageName(pkg string, packageNames []string) string {
	if strings.HasSuffix(pkg, ".32bit") {
		basePkg := strings.TrimSuffix(pkg, ".32bit")
		for _, validPkg := range packageNames {
			if validPkg == basePkg {
				return basePkg
			}
		}
	}

	return pkg
}

// updateInstalledInfo обновляет срез пакетов, устанавливая поля Installed и InstalledVersion, если пакет найден в системе.
func (a *Actions) updateInstalledInfo(packages []Package) ([]Package, error) {
	installed, err := a.GetInstalledPackages()
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
func (a *Actions) GetInstalledPackages() (map[string]string, error) {
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

	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка сканирования вывода rpm: %w", err)
	}

	return installed, nil
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
		return fmt.Errorf(aptError.Error())
	}
	if err != nil {
		return fmt.Errorf("ошибка обновления пакетов: %v, output: %s", err, string(output))
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

func cleanDependency(dep string) string {
	re := regexp.MustCompile(`\s*\(.*?\)`)
	return strings.TrimSpace(re.ReplaceAllString(dep, ""))
}

func parseAptOutput(output string) (PackageChanges, error) {
	pc := &PackageChanges{}
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
