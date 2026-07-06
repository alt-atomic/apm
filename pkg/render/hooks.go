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

package render

// T_ is the string translation hook, defaults to identity
var T_ = func(msgId string) string { return msgId }

// translateKey is the field label hook, defaults to identity
var translateKey = func(key string) string { return key }

// SetTranslator sets the translation function
func SetTranslator(fn func(msgId string) string) {
	if fn != nil {
		T_ = fn
	}
}

// SetKeyTranslator sets the field label translator
func SetKeyTranslator(fn func(key string) string) {
	if fn != nil {
		translateKey = fn
	}
}
