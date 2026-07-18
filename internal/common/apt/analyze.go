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

package apt

import (
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/app"
)

// enrichErrorDetails добавляет детали к ошибке из логов и строк ошибки
func enrichErrorDetails(m *MatchedError, logs []string, errLines []string) {
	if m.IsTransactionError() {
		if details := CollectTransactionDetails(logs); details != "" {
			m.Details = details
		}
	}

	if m.Entry.Code == ErrMultiInstallProvidersSelect && len(errLines) > 1 {
		var providers []string
		for i := 1; i < len(errLines); i++ {
			line := strings.TrimSpace(errLines[i])
			if line != "" && !strings.HasPrefix(line, "You should") {
				providers = append(providers, line)
			}
		}
		if len(providers) > 0 {
			m.Details = strings.Join(providers, "\n") + "\n" + app.T_("You should explicitly select one to install")
		}
	}
}

// analyzeOperation анализ всех ошибок операции, включает в себя stdout из apt-lib
func analyzeOperation(logs []string, err error) error {
	aptErrors := ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		enrichErrorDetails(errApr, logs, nil)
		return errApr
	}

	if err == nil {
		return nil
	}

	if msg := strings.TrimSpace(err.Error()); msg != "" {
		lines := strings.Split(msg, "\n")
		if m := ErrorLinesAnalise(lines); m != nil {
			enrichErrorDetails(m, logs, lines)
			return m
		}
		if m := CheckError(msg); m != nil {
			enrichErrorDetails(m, logs, lines)
			return m
		}
	}

	return err
}

// AnalyzeOperationError разбирает результат APT операции и пишет дамп логов при ошибке
func AnalyzeOperationError(logs []string, err error) error {
	result := analyzeOperation(logs, err)
	if result != nil && len(logs) > 0 {
		app.Log.Error("[APM DUMP ERROR] ", result.Error())
		for _, line := range logs {
			app.Log.Error("[APM DUMP TRACE] ", line)
		}
	}
	return result
}
