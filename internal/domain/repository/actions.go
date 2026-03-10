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

package repository

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/command"
	"apm/internal/domain/repository/service"
	"context"
	"errors"
	"fmt"
	"strings"
)

// ShortRepoResponse Сокращённое представление репозитория
type ShortRepoResponse struct {
	Branch string `json:"branch"`
	URL    string `json:"url"`
	Arch   string `json:"arch"`
}

// FormatRepoOutput принимает данные (один репозиторий или срез) и флаг full.
// Если full == true, возвращается полный вывод, иначе — сокращённый.
func FormatRepoOutput(data interface{}, full bool) interface{} {
	switch v := data.(type) {
	case service.Repository:
		if full {
			return v
		}
		return ShortRepoResponse{
			Branch: v.Branch,
			URL:    v.URL,
			Arch:   v.Arch,
		}
	case []service.Repository:
		if full {
			return v
		}
		shortList := make([]ShortRepoResponse, 0, len(v))
		for _, repo := range v {
			shortList = append(shortList, ShortRepoResponse{
				Branch: repo.Branch,
				URL:    repo.URL,
				Arch:   repo.Arch,
			})
		}
		return shortList
	default:
		return data
	}
}

// Actions объединяет методы для работы с репозиториями
type Actions struct {
	appConfig         *app.Config
	repoService       repoService
	serviceAptActions aptActionsService
}

// NewActions создаёт новый экземпляр Actions
func NewActions(appConfig *app.Config) *Actions {
	packageDBSvc := _package.NewPackageDBService(appConfig.DatabaseManager)
	aptActions := _package.NewActions(packageDBSvc, appConfig)

	cfg := appConfig.ConfigManager.GetConfig()
	runner := command.NewRunner(cfg.CommandPrefix, cfg.Verbose)

	return &Actions{
		appConfig:         appConfig,
		repoService:       service.NewRepoService(packageDBSvc, runner),
		serviceAptActions: aptActions,
	}
}

// List возвращает список репозиториев
func (a *Actions) List(ctx context.Context, all bool) (*RepoListResponse, error) {
	repos, err := a.repoService.GetRepositories(ctx, all)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	var message string
	if all {
		message = fmt.Sprintf(app.TN_("%d repository found (including inactive)", "%d repositories found (including inactive)", len(repos)), len(repos))
	} else {
		message = fmt.Sprintf(app.TN_("%d active repository found", "%d active repositories found", len(repos)), len(repos))
	}

	return &RepoListResponse{
		Message:      message,
		Repositories: repos,
		Count:        len(repos),
	}, nil
}

// Add добавляет репозиторий
// args: [source] или [type, url, arch, components...]
func (a *Actions) Add(ctx context.Context, args []string, date string) (*RepoAddRemoveResponse, error) {
	if len(args) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Repository source must be specified")))
	}
	date = strings.TrimSpace(date)

	added, err := a.repoService.AddRepository(ctx, args, date)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(added) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("All repositories already exist")))
	}

	message := fmt.Sprintf(app.TN_("%d repository added", "%d repositories added", len(added)), len(added))

	return &RepoAddRemoveResponse{
		Message: message,
		Added:   added,
	}, nil
}

// CheckAdd симулирует добавление репозитория
func (a *Actions) CheckAdd(ctx context.Context, args []string, date string) (*RepoSimulateResponse, error) {
	if len(args) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Repository source must be specified")))
	}
	date = strings.TrimSpace(date)

	willAdd, err := a.repoService.SimulateAdd(ctx, args, date, false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(willAdd) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("All repositories already exist")))
	}

	return &RepoSimulateResponse{
		Message: app.T_("Simulation results"),
		WillAdd: willAdd,
	}, nil
}

// Remove удаляет репозиторий
// args: [source] или [type, url, arch, components...]
func (a *Actions) Remove(ctx context.Context, args []string, date string) (*RepoAddRemoveResponse, error) {
	if len(args) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Repository source must be specified")))
	}
	date = strings.TrimSpace(date)

	removed, err := a.repoService.RemoveRepository(ctx, args, date, false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(removed) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("No repositories found to remove")))
	}

	message := fmt.Sprintf(app.TN_("%d repository removed", "%d repositories removed", len(removed)), len(removed))

	return &RepoAddRemoveResponse{
		Message: message,
		Removed: removed,
	}, nil
}

// CheckRemove симулирует удаление репозитория
func (a *Actions) CheckRemove(ctx context.Context, args []string, date string) (*RepoSimulateResponse, error) {
	if len(args) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Repository source must be specified")))
	}
	date = strings.TrimSpace(date)

	willRemove, err := a.repoService.SimulateRemove(ctx, args, date, false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(willRemove) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("No repositories to remove")))
	}

	return &RepoSimulateResponse{
		Message:    app.T_("Simulation results"),
		WillRemove: willRemove,
	}, nil
}

// Set устанавливает ветку (удаляет все и добавляет)
func (a *Actions) Set(ctx context.Context, branch, date string) (*RepoSetResponse, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Branch name must be specified")))
	}
	date = strings.TrimSpace(date)

	added, removed, err := a.repoService.SetBranch(ctx, branch, date)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	// Формируем имя ветки для сообщения
	branchDisplay := branch
	if date != "" {
		branchDisplay = branch + " " + date
	}
	message := fmt.Sprintf(app.T_("Branch %s set successfully"), branchDisplay)

	return &RepoSetResponse{
		Message: message,
		Branch:  branchDisplay,
		Added:   added,
		Removed: removed,
	}, nil
}

// CheckSet симулирует установку ветки
func (a *Actions) CheckSet(ctx context.Context, branch, date string) (*RepoSimulateResponse, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Branch name must be specified")))
	}
	date = strings.TrimSpace(date)

	willRemove, err := a.repoService.SimulateRemove(ctx, []string{"all"}, "", false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}
	willAdd, err := a.repoService.SimulateAdd(ctx, []string{branch}, date, true)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	return &RepoSimulateResponse{
		Message:    app.T_("Simulation results"),
		WillAdd:    willAdd,
		WillRemove: willRemove,
	}, nil
}

// Clean удаляет cdrom и task репозитории
func (a *Actions) Clean(ctx context.Context) (*RepoAddRemoveResponse, error) {
	removed, err := a.repoService.CleanTemporary(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(removed) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("No cdrom or task repositories found")))
	}

	message := fmt.Sprintf(app.TN_("%d temporary repository removed", "%d temporary repositories removed", len(removed)), len(removed))

	return &RepoAddRemoveResponse{
		Message: message,
		Removed: removed,
	}, nil
}

// CheckClean симулирует очистку cdrom и task репозиториев
func (a *Actions) CheckClean(ctx context.Context) (*RepoSimulateResponse, error) {
	repos, err := a.repoService.GetRepositories(ctx, false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	var willRemove []service.Repository
	for _, repo := range repos {
		isCdrom := strings.Contains(repo.URL, "cdrom:")
		isTask := false
		for _, comp := range repo.Components {
			if comp == "task" {
				isTask = true
				break
			}
		}
		if isCdrom || isTask {
			willRemove = append(willRemove, repo)
		}
	}

	if len(willRemove) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("No cdrom or task repositories to remove")))
	}

	return &RepoSimulateResponse{
		Message:    app.T_("Simulation results"),
		WillRemove: willRemove,
	}, nil
}

// GetBranches возвращает список доступных веток
func (a *Actions) GetBranches(_ context.Context) (*BranchesResponse, error) {
	branches := a.repoService.GetBranches()

	return &BranchesResponse{
		Message:  app.T_("Available branches"),
		Branches: branches,
	}, nil
}

// GetTaskPackages возвращает список пакетов из задачи
func (a *Actions) GetTaskPackages(ctx context.Context, taskNum string) (*TaskPackagesResponse, error) {
	taskNum = strings.TrimSpace(taskNum)
	if taskNum == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Task number must be specified")))
	}

	packages, err := a.repoService.GetTaskPackages(ctx, taskNum)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	message := fmt.Sprintf(app.TN_("%d package in task %s", "%d packages in task %s", len(packages)), len(packages), taskNum)

	return &TaskPackagesResponse{
		Message:  message,
		TaskNum:  taskNum,
		Packages: packages,
		Count:    len(packages),
	}, nil
}

// GenerateOnlineDoc запускает веб-сервер с HTML документацией для DBus API
func (a *Actions) GenerateOnlineDoc(ctx context.Context) error {
	return startDocServer(ctx)
}

// TestTask тестирует пакеты из задачи
func (a *Actions) TestTask(ctx context.Context, taskNum string) (*TestTaskResponse, error) {
	taskNum = strings.TrimSpace(taskNum)
	if taskNum == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Task number must be specified")))
	}

	var packagesToInstall []string
	var err error

	packagesToInstall, err = a.repoService.GetTaskPackages(ctx, taskNum)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, err)
	}

	if len(packagesToInstall) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("No packages to install from task")))
	}

	_, err = a.repoService.AddRepository(ctx, []string{taskNum}, "")
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeRepository, fmt.Errorf("%s: %v", app.T_("Failed to add task repository"), err))
	}

	defer func() {
		_, _ = a.repoService.RemoveRepository(ctx, []string{taskNum}, "", false)
	}()

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	packagesInstall, packagesRemove, _, packageParse, errFind := a.serviceAptActions.FindPackage(
		ctx,
		packagesToInstall,
		nil,
		false,
		false,
		false,
	)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("The operation will not make any changes")))
	}

	err = a.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove, false, false, false)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	message := fmt.Sprintf(
		"%s %s %s (%s %s)",
		fmt.Sprintf(app.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
		app.T_("and"),
		fmt.Sprintf(app.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
		app.T_("task"),
		taskNum,
	)

	return &TestTaskResponse{
		Message: message,
		TaskNum: taskNum,
		Info:    *packageParse,
	}, nil
}
