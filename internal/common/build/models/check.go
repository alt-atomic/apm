/*
 * Copyright (C) 2025 Vladimir Romanov <rirusha@altlinux.org>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see
 * <https://www.gnu.org/licenses/gpl-3.0-standalone.html>.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

package models

import (
	"apm/internal/common/app"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

func KebabToPascal(s string) string {
	if len(s) == 0 {
		return s
	}

	var result []rune
	capitalizeNext := true // Флаг: следующую букву нужно сделать заглавной

	for _, r := range s {
		if r == '-' {
			capitalizeNext = true // После дефиса — заглавная
		} else {
			if capitalizeNext {
				result = append(result, unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				result = append(result, unicode.ToLower(r))
			}
		}
	}

	return string(result)
}

func pascalToKebab(s string) string {
	if len(s) == 0 {
		return s
	}

	var result []rune

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, '-')
		}
		result = append(result, unicode.ToLower(r))
	}

	return string(result)
}

func checkRequired(parent reflect.Value, field reflect.Value, fieldType reflect.StructField) error {
	// Required equal something or not present at all
	_, ok := fieldType.Tag.Lookup("required")
	if ok {
		if field.IsZero() {
			return fmt.Errorf(
				"'%s' required in '%s' type but not present",
				pascalToKebab(fieldType.Name),
				BodyTypeToType(parent.Type().Name()),
			)
		}
	}

	return nil
}

func checkDepricated(parent reflect.Value, _ reflect.Value, fieldType reflect.StructField) error {
	// Required equal something or not present at all
	value, ok := fieldType.Tag.Lookup("depricated")
	if ok {
		app.Log.Warning(fmt.Sprintf(
			"'%s' in '%s' is depricated and will be dropped in %s",
			pascalToKebab(fieldType.Name),
			BodyTypeToType(parent.Type().Name()),
			value,
		))
	}

	return nil
}

func BodyTypeToType(bodyType string) string {
	return pascalToKebab(strings.TrimSuffix(bodyType, "Body"))
}

func checkNeeds(parent reflect.Value, field reflect.Value, fieldType reflect.StructField) error {
	// We dont touch needs field if it epmpty
	if field.IsZero() {
		return nil
	}
	whatNeeds, ok := fieldType.Tag.Lookup("needs")
	if ok {
		needsField := parent.FieldByName(whatNeeds)
		if needsField.IsZero() {
			return fmt.Errorf(
				"'%s' needs '%s' in '%s' type but it not present",
				pascalToKebab(fieldType.Name),
				pascalToKebab(whatNeeds),
				BodyTypeToType(parent.Type().Name()),
			)
		}
	}

	return nil
}

func checkConflicts(parent reflect.Value, field reflect.Value, fieldType reflect.StructField) error {
	// We dont touch needs field if it epmpty
	if field.IsZero() {
		return nil
	}
	conflictWith, ok := fieldType.Tag.Lookup("conflicts")
	if ok {
		conflictField := parent.FieldByName(conflictWith)
		if !conflictField.IsZero() {
			return fmt.Errorf(
				"'%s' conflicts with '%s' in '%s' type which present",
				pascalToKebab(fieldType.Name),
				pascalToKebab(conflictWith),
				BodyTypeToType(parent.Type().Name()),
			)
		}
	}

	return nil
}

func CheckBody(v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer {
		panic("ResolveStruct requires a pointer to struct")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		panic("ResolveStruct requires a pointer to struct")
	}

	return CheckBodyValue(val)
}

func CheckBodyValue(val reflect.Value) error {
	valType := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := valType.Field(i)

		if err := checkRequired(val, field, fieldType); err != nil {
			return err
		}
		if err := checkNeeds(val, field, fieldType); err != nil {
			return err
		}
		if err := checkConflicts(val, field, fieldType); err != nil {
			return err
		}
		if err := checkDepricated(val, field, fieldType); err != nil {
			return err
		}
	}

	return nil
}
