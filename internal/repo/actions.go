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

package repo

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/reply"
	"apm/internal/repo/service"
	"context"
	"errors"
	"fmt"
	"strings"
)

// Actions объединяет методы для работы с репозиториями
type Actions struct {
	appConfig         *app.Config
	repoService       *service.RepoService
	serviceAptActions *_package.Actions
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручным управлением зависимостями
func NewActionsWithDeps(
	appConfig *app.Config,
	repoService *service.RepoService,
	aptActions *_package.Actions,
) *Actions {
	return &Actions{
		appConfig:         appConfig,
		repoService:       repoService,
		serviceAptActions: aptActions,
	}
}

// NewActions создаёт новый экземпляр Actions
func NewActions(appConfig *app.Config) *Actions {
	packageDBSvc := _package.NewPackageDBService(appConfig.DatabaseManager)
	aptActions := _package.NewActions(packageDBSvc, appConfig)

	return &Actions{
		appConfig:         appConfig,
		repoService:       service.NewRepoService(appConfig),
		serviceAptActions: aptActions,
	}
}

// List возвращает список репозиториев
func (a *Actions) List(ctx context.Context, all bool) (*reply.APIResponse, error) {
	repos, err := a.repoService.GetRepositories(ctx, all)
	if err != nil {
		return nil, err
	}

	var message string
	if all {
		message = fmt.Sprintf(app.TN_("%d repository found (including inactive)", "%d repositories found (including inactive)", len(repos)), len(repos))
	} else {
		message = fmt.Sprintf(app.TN_("%d active repository found", "%d active repositories found", len(repos)), len(repos))
	}

	return &reply.APIResponse{
		Data: ListResponse{
			Message:      message,
			Repositories: repos,
			Count:        len(repos),
		},
		Error: false,
	}, nil
}

// Add добавляет репозиторий
// args: [source] или [type, url, arch, components...]
func (a *Actions) Add(ctx context.Context, args []string, date string) (*reply.APIResponse, error) {
	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}
	date = strings.TrimSpace(date)

	added, err := a.repoService.AddRepository(ctx, args, date)
	if err != nil {
		return nil, err
	}

	if len(added) == 0 {
		return &reply.APIResponse{
			Data: AddRemoveResponse{
				Message: app.T_("All repositories already exist"),
				Added:   []string{},
			},
			Error: false,
		}, nil
	}

	message := fmt.Sprintf(app.TN_("%d repository added", "%d repositories added", len(added)), len(added))

	return &reply.APIResponse{
		Data: AddRemoveResponse{
			Message: message,
			Added:   added,
		},
		Error: false,
	}, nil
}

// CheckAdd симулирует добавление репозитория
func (a *Actions) CheckAdd(ctx context.Context, args []string, date string) (*reply.APIResponse, error) {
	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}
	date = strings.TrimSpace(date)

	willAdd, err := a.repoService.SimulateAdd(ctx, args, date)
	if err != nil {
		return nil, err
	}

	if len(willAdd) == 0 {
		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("All repositories already exist"),
			},
			Error: false,
		}, nil
	}

	return &reply.APIResponse{
		Data: SimulateResponse{
			Message: app.T_("Simulation results"),
			WillAdd: willAdd,
		},
		Error: false,
	}, nil
}

// Remove удаляет репозиторий
// args: [source] или [type, url, arch, components...]
func (a *Actions) Remove(ctx context.Context, args []string, date string) (*reply.APIResponse, error) {
	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}
	date = strings.TrimSpace(date)

	removed, err := a.repoService.RemoveRepository(ctx, args, date, false)
	if err != nil {
		return nil, err
	}

	if len(removed) == 0 {
		return &reply.APIResponse{
			Data: AddRemoveResponse{
				Message: app.T_("No repositories found to remove"),
				Removed: []string{},
			},
			Error: false,
		}, nil
	}

	message := fmt.Sprintf(app.TN_("%d repository removed", "%d repositories removed", len(removed)), len(removed))

	return &reply.APIResponse{
		Data: AddRemoveResponse{
			Message: message,
			Removed: removed,
		},
		Error: false,
	}, nil
}

// CheckRemove симулирует удаление репозитория
func (a *Actions) CheckRemove(ctx context.Context, args []string, date string) (*reply.APIResponse, error) {
	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}
	date = strings.TrimSpace(date)

	willRemove, err := a.repoService.SimulateRemove(ctx, args, date, false)
	if err != nil {
		return nil, err
	}

	if len(willRemove) == 0 {
		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("No repositories to remove"),
			},
			Error: false,
		}, nil
	}

	return &reply.APIResponse{
		Data: SimulateResponse{
			Message:    app.T_("Simulation results"),
			WillRemove: willRemove,
		},
		Error: false,
	}, nil
}

// Set устанавливает ветку (удаляет все и добавляет)
func (a *Actions) Set(ctx context.Context, branch, date string) (*reply.APIResponse, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, errors.New(app.T_("Branch name must be specified"))
	}
	date = strings.TrimSpace(date)

	added, removed, err := a.repoService.SetBranch(ctx, branch, date)
	if err != nil {
		return nil, err
	}

	// Формируем имя ветки для сообщения
	branchDisplay := branch
	if date != "" {
		branchDisplay = branch + " " + date
	}
	message := fmt.Sprintf(app.T_("Branch %s set successfully"), branchDisplay)

	return &reply.APIResponse{
		Data: SetResponse{
			Message: message,
			Branch:  branchDisplay,
			Added:   added,
			Removed: removed,
		},
		Error: false,
	}, nil
}

// CheckSet симулирует установку ветки
func (a *Actions) CheckSet(ctx context.Context, branch, date string) (*reply.APIResponse, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, errors.New(app.T_("Branch name must be specified"))
	}
	date = strings.TrimSpace(date)

	// Симулируем удаление всех веток
	willRemove, err := a.repoService.SimulateRemove(ctx, []string{"all"}, "", false)
	if err != nil {
		return nil, err
	}

	// Симулируем добавление ветки
	willAdd, err := a.repoService.SimulateAdd(ctx, []string{branch}, date)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: SimulateResponse{
			Message:    app.T_("Simulation results"),
			WillAdd:    willAdd,
			WillRemove: willRemove,
		},
		Error: false,
	}, nil
}

// Clean удаляет cdrom и task репозитории
func (a *Actions) Clean(ctx context.Context) (*reply.APIResponse, error) {
	removed, err := a.repoService.CleanTemporary(ctx)
	if err != nil {
		return nil, err
	}

	if len(removed) == 0 {
		return &reply.APIResponse{
			Data: AddRemoveResponse{
				Message: app.T_("No cdrom or task repositories found"),
				Removed: []string{},
			},
			Error: false,
		}, nil
	}

	message := fmt.Sprintf(app.TN_("%d temporary repository removed", "%d temporary repositories removed", len(removed)), len(removed))

	return &reply.APIResponse{
		Data: AddRemoveResponse{
			Message: message,
			Removed: removed,
		},
		Error: false,
	}, nil
}

// CheckClean симулирует очистку cdrom и task репозиториев
func (a *Actions) CheckClean(ctx context.Context) (*reply.APIResponse, error) {
	repos, err := a.repoService.GetRepositories(ctx, false)
	if err != nil {
		return nil, err
	}

	var willRemove []string
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
			willRemove = append(willRemove, repo.Entry)
		}
	}

	if len(willRemove) == 0 {
		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("No cdrom or task repositories to remove"),
			},
			Error: false,
		}, nil
	}

	return &reply.APIResponse{
		Data: SimulateResponse{
			Message:    app.T_("Simulation results"),
			WillRemove: willRemove,
		},
		Error: false,
	}, nil
}

// GetBranches возвращает список доступных веток
func (a *Actions) GetBranches(_ context.Context) (*reply.APIResponse, error) {
	branches := a.repoService.GetBranches()

	return &reply.APIResponse{
		Data: BranchesResponse{
			Message:  app.T_("Available branches"),
			Branches: branches,
		},
		Error: false,
	}, nil
}

// GetTaskPackages возвращает список пакетов из задачи
func (a *Actions) GetTaskPackages(ctx context.Context, taskNum string) (*reply.APIResponse, error) {
	taskNum = strings.TrimSpace(taskNum)
	if taskNum == "" {
		return nil, errors.New(app.T_("Task number must be specified"))
	}

	packages, err := a.repoService.GetTaskPackages(ctx, taskNum)
	if err != nil {
		return nil, err
	}

	message := fmt.Sprintf(app.TN_("%d package in task %s", "%d packages in task %s", len(packages)), len(packages), taskNum)

	return &reply.APIResponse{
		Data: TaskPackagesResponse{
			Message:  message,
			TaskNum:  taskNum,
			Packages: packages,
			Count:    len(packages),
		},
		Error: false,
	}, nil
}

// GenerateOnlineDoc запускает веб-сервер с HTML документацией для DBus API
func (a *Actions) GenerateOnlineDoc(ctx context.Context) error {
	return startDocServer(ctx)
}

// TestTask тестирует пакеты из задачи
func (a *Actions) TestTask(ctx context.Context, taskNum string) (*reply.APIResponse, error) {
	taskNum = strings.TrimSpace(taskNum)
	if taskNum == "" {
		return nil, errors.New(app.T_("Task number must be specified"))
	}

	var packagesToInstall []string
	var err error

	packagesToInstall, err = a.repoService.GetTaskPackages(ctx, taskNum)
	if err != nil {
		return nil, err
	}

	if len(packagesToInstall) == 0 {
		return nil, errors.New(app.T_("No packages to install from task"))
	}

	_, err = a.repoService.AddRepository(ctx, []string{taskNum}, "")
	if err != nil {
		return nil, fmt.Errorf("%s: %v", app.T_("Failed to add task repository"), err)
	}

	defer func() {
		_, _ = a.repoService.RemoveRepository(ctx, []string{taskNum}, "", false)
	}()

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
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
		return nil, errFind
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": app.T_("The operation will not make any changes"),
				"taskNum": taskNum,
			},
			Error: false,
		}, nil
	}

	err = a.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove, false, false)
	if err != nil {
		return nil, err
	}

	message := fmt.Sprintf(
		"%s %s %s (%s %s)",
		fmt.Sprintf(app.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
		app.T_("and"),
		fmt.Sprintf(app.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
		app.T_("task"),
		taskNum,
	)

	return &reply.APIResponse{
		Data: TestTaskResponse{
			Message: message,
			TaskNum: taskNum,
			Info:    *packageParse,
		},
		Error: false,
	}, nil
}
