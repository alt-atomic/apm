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

package _package

import (
	"apm/internal/common/app"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"fmt"
	"sync"
	"time"
)

// progressThrottler управляет throttling-ом для прогресс-событий.
type progressThrottler struct {
	lastPercent int
	lastUpdate  time.Time
}

func newProgressThrottler() progressThrottler {
	return progressThrottler{lastPercent: -1, lastUpdate: time.Now()}
}

// ShouldUpdate возвращает true, если прошло достаточно времени для обновления прогресса.
func (pt *progressThrottler) ShouldUpdate(percent int) bool {
	if pt.lastPercent == -1 {
		return true
	}
	if percent == pt.lastPercent {
		return false
	}
	elapsed := time.Since(pt.lastUpdate)
	if percent < 10 || percent > 90 {
		return elapsed >= 50*time.Millisecond
	}
	return elapsed >= 100*time.Millisecond
}

// RecordUpdate фиксирует текущее значение процента и время обновления.
func (pt *progressThrottler) RecordUpdate(percent int) {
	pt.lastPercent = percent
	pt.lastUpdate = time.Now()
}

func (a *Actions) getHandler(ctx context.Context, packageCount ...int) func(pkg string, event aptLib.ProgressType, cur, total, speed uint64) {
	pkgCount := 0
	if len(packageCount) > 0 {
		pkgCount = packageCount[0]
	}
	// Состояние для загрузки
	downloadThrottle := newProgressThrottler()
	downloadStarted := false

	// Состояние для установки пакетов
	packageState := make(map[string]*packageProgress)
	var packageMutex sync.Mutex
	nextInstallID := 1

	return func(pkg string, event aptLib.ProgressType, cur, total, speed uint64) {
		switch event {
		case aptLib.CallbackDownloadProgress:
			if total == 0 {
				return
			}
			downloadStarted = true
			percent := int((cur * 100) / total)

			if percent < 100 && downloadThrottle.ShouldUpdate(percent) {
				downloadThrottle.RecordUpdate(percent)

				viewText := app.T_("Downloading packages")
				if speedStr := helper.FormatSpeed(speed); speedStr != "" {
					viewText += "  " + speedStr
				}

				reply.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(reply.EventSystemDownloadProgress),
					reply.WithProgress(true),
					reply.WithProgressPercent(float64(percent)),
					reply.WithEventView(viewText),
				)
			}
		case aptLib.CallbackDownloadComplete:
			if !downloadStarted {
				return
			}
			doneText := app.T_("All packages downloaded")
			if pkgCount == 1 {
				doneText = app.T_("Package downloaded")
			}
			reply.CreateEventNotification(ctx, reply.StateAfter,
				reply.WithEventName(reply.EventSystemDownloadProgress),
				reply.WithProgress(true),
				reply.WithProgressPercent(100),
				reply.WithProgressDoneText(doneText),
			)
		case aptLib.CallbackInstallProgress:
			if pkg == "" || total == 0 {
				return
			}

			packageMutex.Lock()
			defer packageMutex.Unlock()

			state, exists := packageState[pkg]
			if !exists {
				state = &packageProgress{
					lastPercent: -1,
					lastUpdate:  time.Now(),
					id:          nextInstallID,
				}
				nextInstallID++
				packageState[pkg] = state
			}

			percent := int((cur * 100) / total)
			now := time.Now()
			elapsed := now.Sub(state.lastUpdate)

			// Throttling для установки пакетов (другой алгоритм: percentDiff 3 уровня)
			shouldUpdate := false

			if state.lastPercent == -1 {
				shouldUpdate = true
			} else if percent == 100 {
				shouldUpdate = true
			} else if percent != state.lastPercent {
				percentDiff := helper.Abs(percent - state.lastPercent)

				if percentDiff >= 10 {
					shouldUpdate = elapsed >= 50*time.Millisecond
				} else if percentDiff >= 5 {
					shouldUpdate = elapsed >= 100*time.Millisecond
				} else {
					shouldUpdate = elapsed >= 200*time.Millisecond
				}
			}

			if shouldUpdate {
				state.lastPercent = percent
				state.lastUpdate = now

				ev := fmt.Sprintf("%s-%d", reply.EventSystemInstallProgress, state.id)

				if percent < 100 {
					reply.CreateEventNotification(ctx, reply.StateBefore,
						reply.WithEventName(ev),
						reply.WithProgress(true),
						reply.WithProgressPercent(float64(percent)),
						reply.WithEventView(fmt.Sprintf(app.T_("Installing progress: %s"), pkg)),
					)
				} else {
					reply.CreateEventNotification(ctx, reply.StateAfter,
						reply.WithEventName(ev),
						reply.WithProgress(true),
						reply.WithProgressPercent(100),
						reply.WithEventView(fmt.Sprintf(app.T_("Installing %s"), pkg)),
						reply.WithProgressDoneText(fmt.Sprintf(app.T_("Installing %s"), pkg)),
					)

					// Удаляем из отслеживания
					delete(packageState, pkg)
				}
			}
		}
	}
}

func (a *Actions) getUpdateHandler(ctx context.Context) aptLib.ProgressHandler {
	type itemState struct {
		progressThrottler
		id int
	}
	items := make(map[string]*itemState)
	done := make(map[string]bool)
	var mu sync.Mutex
	nextID := 1

	return func(pkg string, event aptLib.ProgressType, cur, total, speed uint64) {
		switch event {
		case aptLib.CallbackDownloadItemProgress:
			if pkg == "" || total == 0 {
				return
			}

			mu.Lock()
			if done[pkg] {
				mu.Unlock()
				return
			}
			state, exists := items[pkg]
			if !exists {
				state = &itemState{
					progressThrottler: newProgressThrottler(),
					id:                nextID,
				}
				nextID++
				items[pkg] = state
			}

			percent := int((cur * 100) / total)

			if percent < 100 && state.ShouldUpdate(percent) {
				state.RecordUpdate(percent)
				id := state.id
				mu.Unlock()

				viewText := pkg
				if speedStr := helper.FormatSpeed(speed); speedStr != "" {
					viewText += "  " + speedStr
				}

				ev := fmt.Sprintf("%s-%d", reply.EventSystemAptUpdate, id)
				reply.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(ev),
					reply.WithProgress(true),
					reply.WithProgressPercent(float64(percent)),
					reply.WithEventView(viewText),
				)
			} else {
				mu.Unlock()
			}

		case aptLib.CallbackDownloadStop:
			mu.Lock()
			state, tracked := items[pkg]
			var id int
			if tracked {
				id = state.id
			}
			delete(items, pkg)
			if tracked {
				done[pkg] = true
			}
			mu.Unlock()

			if tracked && pkg != "" {
				ev := fmt.Sprintf("%s-%d", reply.EventSystemAptUpdate, id)
				reply.CreateEventNotification(ctx, reply.StateAfter,
					reply.WithEventName(ev),
					reply.WithProgress(true),
					reply.WithProgressPercent(100),
					reply.WithEventView(pkg),
					reply.WithProgressDoneText(pkg),
				)
			}
		}
	}
}
