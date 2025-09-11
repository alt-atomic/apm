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

package service

import (
	"testing"
)

// TestIsAllowedField проверяет валидацию разрешенных полей для фильтрации и сортировки
func TestIsAllowedField(t *testing.T) {
	// Создаем экземпляр для тестирования
	service := &DistroDBService{}

	tests := []struct {
		name     string
		field    string
		allowed  []string
		expected bool
	}{
		{
			name:     "Valid field in allowed list",
			field:    "name",
			allowed:  []string{"name", "version", "description"},
			expected: true,
		},
		{
			name:     "Invalid field not in allowed list",
			field:    "invalid_field",
			allowed:  []string{"name", "version", "description"},
			expected: false,
		},
		{
			name:     "Empty field",
			field:    "",
			allowed:  []string{"name", "version", "description"},
			expected: false,
		},
		{
			name:     "Field with exact match",
			field:    "version",
			allowed:  []string{"name", "version", "description"},
			expected: true,
		},
		{
			name:     "Case sensitive field - lowercase in list",
			field:    "Name",
			allowed:  []string{"name", "version", "description"},
			expected: false,
		},
		{
			name:     "Case sensitive field - uppercase in list",
			field:    "name",
			allowed:  []string{"Name", "Version", "Description"},
			expected: false,
		},
		{
			name:     "Empty allowed list",
			field:    "name",
			allowed:  []string{},
			expected: false,
		},
		{
			name:     "Single item allowed list - match",
			field:    "name",
			allowed:  []string{"name"},
			expected: true,
		},
		{
			name:     "Single item allowed list - no match",
			field:    "version",
			allowed:  []string{"name"},
			expected: false,
		},
		{
			name:     "Field with spaces",
			field:    "name ",
			allowed:  []string{"name", "version", "description"},
			expected: false,
		},
		{
			name:     "Field with special characters",
			field:    "name-special",
			allowed:  []string{"name-special", "version", "description"},
			expected: true,
		},
		{
			name:     "Unicode field name",
			field:    "имя",
			allowed:  []string{"имя", "версия", "описание"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isAllowedField(tt.field, tt.allowed)
			if result != tt.expected {
				t.Errorf("isAllowedField(%q, %v) = %v, want %v", tt.field, tt.allowed, result, tt.expected)
			}
		})
	}
}

// TestIsAllowedFieldWithRealData проверяет работу с реальными константами из кода
func TestIsAllowedFieldWithRealData(t *testing.T) {
	service := &DistroDBService{}

	// Тестируем с реальными allowedSortFields
	validSortFields := []string{"name", "version", "description", "container", "installed", "exporting", "manager"}
	invalidSortFields := []string{"invalid", "unknown", "password", "secret", "admin"}

	for _, field := range validSortFields {
		t.Run("ValidSort_"+field, func(t *testing.T) {
			result := service.isAllowedField(field, allowedSortFields)
			if !result {
				t.Errorf("isAllowedField(%q, allowedSortFields) = false, want true", field)
			}
		})
	}

	for _, field := range invalidSortFields {
		t.Run("InvalidSort_"+field, func(t *testing.T) {
			result := service.isAllowedField(field, allowedSortFields)
			if result {
				t.Errorf("isAllowedField(%q, allowedSortFields) = true, want false", field)
			}
		})
	}

	// Тестируем с реальными AllowedFilterFields
	validFilterFields := []string{"name", "version", "description", "container", "installed", "exporting", "manager"}
	invalidFilterFields := []string{"password", "token", "key", "private", "internal"}

	for _, field := range validFilterFields {
		t.Run("ValidFilter_"+field, func(t *testing.T) {
			result := service.isAllowedField(field, AllowedFilterFields)
			if !result {
				t.Errorf("isAllowedField(%q, AllowedFilterFields) = false, want true", field)
			}
		})
	}

	for _, field := range invalidFilterFields {
		t.Run("InvalidFilter_"+field, func(t *testing.T) {
			result := service.isAllowedField(field, AllowedFilterFields)
			if result {
				t.Errorf("isAllowedField(%q, AllowedFilterFields) = true, want false", field)
			}
		})
	}
}

// TestIsAllowedFieldConsistency проверяет согласованность между allowedSortFields и AllowedFilterFields
func TestIsAllowedFieldConsistency(t *testing.T) {
	// Проверяем, что списки содержат одинаковые поля (в данном случае они должны быть идентичными)
	if len(allowedSortFields) != len(AllowedFilterFields) {
		t.Errorf("allowedSortFields and AllowedFilterFields have different lengths: %d vs %d",
			len(allowedSortFields), len(AllowedFilterFields))
	}

	service := &DistroDBService{}

	// Проверяем, что каждое поле из allowedSortFields есть в AllowedFilterFields
	for _, sortField := range allowedSortFields {
		if !service.isAllowedField(sortField, AllowedFilterFields) {
			t.Errorf("Sort field %q is not present in AllowedFilterFields", sortField)
		}
	}

	// Проверяем, что каждое поле из AllowedFilterFields есть в allowedSortFields
	for _, filterField := range AllowedFilterFields {
		if !service.isAllowedField(filterField, allowedSortFields) {
			t.Errorf("Filter field %q is not present in allowedSortFields", filterField)
		}
	}
}

// TestIsAllowedFieldPerformance проверяет производительность функции на больших списках
func TestIsAllowedFieldPerformance(t *testing.T) {
	service := &DistroDBService{}

	// Создаем большой список разрешенных полей
	largeAllowedList := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		largeAllowedList[i] = "field_" + string(rune(i))
	}

	testFields := []string{
		"field_0",     // первый элемент
		"field_500",   // средний элемент
		"field_999",   // последний элемент
		"nonexistent", // несуществующий элемент
	}

	for _, field := range testFields {
		t.Run("Performance_"+field, func(t *testing.T) {
			// Запускаем функцию много раз для проверки производительности
			for i := 0; i < 1000; i++ {
				service.isAllowedField(field, largeAllowedList)
			}
		})
	}
}

// TestIsAllowedFieldEdgeCases проверяет граничные случаи
func TestIsAllowedFieldEdgeCases(t *testing.T) {
	service := &DistroDBService{}

	edgeCases := []struct {
		name     string
		field    string
		allowed  []string
		expected bool
	}{
		{
			name:     "Nil allowed list",
			field:    "name",
			allowed:  nil,
			expected: false,
		},
		{
			name:     "Very long field name",
			field:    string(make([]rune, 1000)),
			allowed:  []string{"name"},
			expected: false,
		},
		{
			name:     "Field with null character",
			field:    "name\x00",
			allowed:  []string{"name"},
			expected: false,
		},
		{
			name:     "Field with newline",
			field:    "name\n",
			allowed:  []string{"name"},
			expected: false,
		},
		{
			name:     "Allowed list with duplicates",
			field:    "name",
			allowed:  []string{"name", "name", "version"},
			expected: true,
		},
		{
			name:     "Empty strings in allowed list",
			field:    "",
			allowed:  []string{"", "name", "version"},
			expected: true,
		},
	}

	for _, tt := range edgeCases {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isAllowedField(tt.field, tt.allowed)
			if result != tt.expected {
				t.Errorf("isAllowedField(%q, %v) = %v, want %v", tt.field, tt.allowed, result, tt.expected)
			}
		})
	}
}
