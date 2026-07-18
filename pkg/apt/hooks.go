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

import "altlinux.space/alt-atomic/apm/pkg/apt/lib"

// T_ is the string translation hook, defaults to identity
var T_ = func(msgId string) string { return msgId }

// errorAnalyzer is the operation error post-processing hook, defaults to pass-through
var errorAnalyzer = func(logs []string, err error) error { return err }

// SetTranslator sets the translation function for the binding and lib
func SetTranslator(fn func(msgId string) string) {
	if fn == nil {
		return
	}
	T_ = fn
	lib.SetTranslator(fn)
}

// SetErrorAnalyzer sets the error analyzer for APT operations
func SetErrorAnalyzer(fn func(logs []string, err error) error) {
	if fn == nil {
		return
	}
	errorAnalyzer = fn
}
