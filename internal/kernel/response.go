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

package kernel

import (
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/kernel/service"
)

// ListKernelsResponse структура ответа для ListKernels метода
type ListKernelsResponse struct {
	Message string                   `json:"message"`
	Kernels []service.FullKernelInfo `json:"kernels"`
}

// GetCurrentKernelResponse структура ответа для GetCurrentKernel метода
type GetCurrentKernelResponse struct {
	Message string                 `json:"message"`
	Kernel  service.FullKernelInfo `json:"kernel"`
}

// InstallUpdateKernelResponse структура ответа для UpdateKernel/InstallKernel методов
type InstallUpdateKernelResponse struct {
	Message string                  `json:"message"`
	Kernel  service.FullKernelInfo  `json:"kernel"`
	Preview *service.UpgradePreview `json:"preview,omitempty"`
}

// WithReasons ядро с причинами сохранения
type WithReasons struct {
	Kernel  service.Info `json:"kernel"`
	Reasons []string     `json:"reasons"`
}

// CleanOldKernelsResponse структура ответа для CleanOldKernels метода
type CleanOldKernelsResponse struct {
	Message       string                 `json:"message"`
	RemoveKernels []service.Info         `json:"removeKernels"`
	KeptKernels   []WithReasons          `json:"keptKernels"`
	Preview       *aptlib.PackageChanges `json:"preview,omitempty"`
}

// ListKernelModulesResponse структура ответа для ListKernelModules метода
type ListKernelModulesResponse struct {
	Message string                 `json:"message"`
	Kernel  service.FullKernelInfo `json:"kernel"`
	Modules []service.ModuleInfo   `json:"modules"`
}

// InstallKernelModulesResponse структура ответа для InstallKernelModules метода
type InstallKernelModulesResponse struct {
	Message string                 `json:"message"`
	Kernel  service.FullKernelInfo `json:"kernel"`
	Preview *aptlib.PackageChanges `json:"preview,omitempty"`
}

// RemoveKernelModulesResponse структура ответа для RemoveKernelModules метода
type RemoveKernelModulesResponse struct {
	Message string                 `json:"message"`
	Kernel  service.FullKernelInfo `json:"kernel"`
	Preview *aptlib.PackageChanges `json:"preview,omitempty"`
}

// BackgroundTaskResponse структура ответа при запуске фоновой задачи
type BackgroundTaskResponse struct {
	Message     string `json:"message"`
	Transaction string `json:"transaction"`
}
