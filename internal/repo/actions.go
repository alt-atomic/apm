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
	"apm/internal/common/reply"
	"apm/internal/repo/service"
	"context"
	"errors"
	"fmt"
	"strings"
)

// Actions объединяет методы для работы с репозиториями
type Actions struct {
	appConfig   *app.Config
	repoService *service.RepoService
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручным управлением зависимостями
func NewActionsWithDeps(
	appConfig *app.Config,
	repoService *service.RepoService,
) *Actions {
	return &Actions{
		appConfig:   appConfig,
		repoService: repoService,
	}
}

// NewActions создаёт новый экземпляр Actions
func NewActions(appConfig *app.Config) *Actions {
	return &Actions{
		appConfig:   appConfig,
		repoService: service.NewRepoService(appConfig),
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
func (a *Actions) Add(ctx context.Context, source string, simulate bool) (*reply.APIResponse, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}

	if simulate {
		willAdd, err := a.repoService.SimulateAdd(ctx, source)
		if err != nil {
			return nil, err
		}

		if len(willAdd) == 0 {
			return &reply.APIResponse{
				Data: SimulateResponse{
					Message: app.T_("All repositories already exist"),
					Changes: []string{},
				},
				Error: false,
			}, nil
		}

		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("Simulation results"),
				Changes: willAdd,
			},
			Error: false,
		}, nil
	}

	added, err := a.repoService.AddRepository(ctx, source)
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

// Remove удаляет репозиторий
func (a *Actions) Remove(ctx context.Context, source string, simulate bool) (*reply.APIResponse, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}

	if simulate {
		willRemove, err := a.repoService.SimulateRemove(ctx, source)
		if err != nil {
			return nil, err
		}

		if len(willRemove) == 0 {
			return &reply.APIResponse{
				Data: SimulateResponse{
					Message: app.T_("No repositories to remove"),
					Changes: []string{},
				},
				Error: false,
			}, nil
		}

		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("Simulation results"),
				Changes: willRemove,
			},
			Error: false,
		}, nil
	}

	removed, err := a.repoService.RemoveRepository(ctx, source)
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

// Set устанавливает ветку (удаляет все и добавляет)
func (a *Actions) Set(ctx context.Context, branch string, simulate bool) (*reply.APIResponse, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, errors.New(app.T_("Branch name must be specified"))
	}

	if simulate {
		// Симулируем удаление всех
		willRemove, err := a.repoService.SimulateRemove(ctx, "all")
		if err != nil {
			return nil, err
		}

		// Симулируем добавление ветки
		willAdd, err := a.repoService.SimulateAdd(ctx, branch)
		if err != nil {
			return nil, err
		}

		changes := append(willRemove, willAdd...)

		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("Simulation results"),
				Changes: changes,
			},
			Error: false,
		}, nil
	}

	added, removed, err := a.repoService.SetBranch(ctx, branch)
	if err != nil {
		return nil, err
	}

	message := fmt.Sprintf(app.T_("Branch %s set successfully"), branch)

	return &reply.APIResponse{
		Data: SetResponse{
			Message: message,
			Branch:  branch,
			Added:   added,
			Removed: removed,
		},
		Error: false,
	}, nil
}

// Clean удаляет cdrom и task репозитории
func (a *Actions) Clean(ctx context.Context, simulate bool) (*reply.APIResponse, error) {
	if simulate {
		// Получаем текущие репозитории и фильтруем cdrom/task
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
				willRemove = append(willRemove, fmt.Sprintf(app.T_("Will remove: %s"), repo.Raw))
			}
		}

		if len(willRemove) == 0 {
			return &reply.APIResponse{
				Data: SimulateResponse{
					Message: app.T_("No cdrom or task repositories to remove"),
					Changes: []string{},
				},
				Error: false,
			}, nil
		}

		return &reply.APIResponse{
			Data: SimulateResponse{
				Message: app.T_("Simulation results"),
				Changes: willRemove,
			},
			Error: false,
		}, nil
	}

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
