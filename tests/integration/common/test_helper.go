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

package common

import (
	"context"
	"testing"

	"apm/internal/common/app"
)

// GetTestAppConfig возвращает инициализированный app.Config для тестов
func GetTestAppConfig(t *testing.T) (*app.Config, context.Context) {
	appConfig, err := app.InitializeAppDefault()
	if err != nil {
		t.Fatalf("Error initializing app config for test: %v", err)
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, app.AppConfigKey, appConfig)

	return appConfig, ctx
}
