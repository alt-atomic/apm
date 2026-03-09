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

package sandbox

import (
	"testing"
)

// TestDistroFilterConfigAllowedFields проверяет валидацию разрешённых полей для фильтрации
func TestDistroFilterConfigAllowedFields(t *testing.T) {
	validFields := []string{"name", "version", "description", "container", "installed", "exporting", "manager"}
	invalidFields := []string{"invalid", "unknown", "password", "secret", "admin"}

	for _, field := range validFields {
		t.Run("Valid_"+field, func(t *testing.T) {
			if !DistroFilterConfig.IsAllowedField(field) {
				t.Errorf("IsAllowedField(%q) = false, want true", field)
			}
		})
	}

	for _, field := range invalidFields {
		t.Run("Invalid_"+field, func(t *testing.T) {
			if DistroFilterConfig.IsAllowedField(field) {
				t.Errorf("IsAllowedField(%q) = true, want false", field)
			}
		})
	}
}

// TestDistroFilterConfigParse проверяет парсинг фильтров через конфигурацию
func TestDistroFilterConfigParse(t *testing.T) {
	t.Run("default op for name is like", func(t *testing.T) {
		filters, err := DistroFilterConfig.Parse([]string{"name=test"})
		if err != nil {
			t.Fatal(err)
		}
		if len(filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(filters))
		}
		if filters[0].Op != "like" {
			t.Errorf("expected op 'like', got %q", filters[0].Op)
		}
	})

	t.Run("explicit eq for name", func(t *testing.T) {
		filters, err := DistroFilterConfig.Parse([]string{"name[eq]=test"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != "eq" {
			t.Errorf("expected op 'eq', got %q", filters[0].Op)
		}
	})

	t.Run("default op for installed is eq", func(t *testing.T) {
		filters, err := DistroFilterConfig.Parse([]string{"installed=true"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != "eq" {
			t.Errorf("expected op 'eq', got %q", filters[0].Op)
		}
	})

	t.Run("disallowed op for installed", func(t *testing.T) {
		_, err := DistroFilterConfig.Parse([]string{"installed[like]=true"})
		if err == nil {
			t.Fatal("expected error for disallowed op on installed")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := DistroFilterConfig.Parse([]string{"unknown=value"})
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})
}

// TestSortAndFilterFieldsConsistency проверяет, что все поля фильтрации являются сортируемыми
func TestSortAndFilterFieldsConsistency(t *testing.T) {
	filterFields := DistroFilterConfig.AllowedFields()
	sortableFields := DistroFilterConfig.SortableFields()

	for _, field := range filterFields {
		if err := DistroFilterConfig.ValidateSortField(field); err != nil {
			t.Errorf("Filter field %q should be sortable but is not", field)
		}
	}

	for _, field := range sortableFields {
		if !DistroFilterConfig.IsAllowedField(field) {
			t.Errorf("Sortable field %q is not a valid filter field", field)
		}
	}
}
