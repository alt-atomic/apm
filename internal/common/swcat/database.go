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

package swcat

import (
	"apm/internal/common/app"
	"apm/internal/common/filter"
	"apm/internal/common/reply"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AppStreamPrefix префикс для фильтрации по полям AppStream компонентов.
const AppStreamPrefix = "app."

// DBAppStream модель для таблицы host_appstream_components.
type DBAppStream struct {
	ID         uint        `gorm:"primaryKey;autoIncrement" json:"id"`
	PkgName    string      `gorm:"column:pkgname;index;not null" json:"pkgname"`
	Components []Component `gorm:"column:components;serializer:json;type:TEXT" json:"components"`
}

func (DBAppStream) TableName() string { return "host_appstream_components" }

// DBService сервис для работы с таблицей AppStream.
type DBService struct {
	dbManager app.DatabaseManager
	realDb    *gorm.DB
	mu        sync.Mutex
}

func NewAppStreamDBService(dbManager app.DatabaseManager) *DBService {
	return &DBService{dbManager: dbManager}
}

func (s *DBService) db() (*gorm.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.realDb == nil {
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{LogLevel: logger.Silent},
		)

		var err error
		s.realDb, err = gorm.Open(sqlite.Dialector{
			Conn:       s.dbManager.GetSystemDB(),
			DriverName: "sqlite3",
		}, &gorm.Config{Logger: gormLogger})
		if err != nil {
			return nil, fmt.Errorf("failed to connect GORM to SQLite: %w", err)
		}

		if err = s.realDb.AutoMigrate(&DBAppStream{}); err != nil {
			return nil, fmt.Errorf("failed to migrate host_appstream_components: %w", err)
		}
	}

	return s.realDb, nil
}

// SaveComponentsToDB полностью перезаписывает таблицу AppStream компонентов.
func (s *DBService) SaveComponentsToDB(ctx context.Context, pkgMap map[string][]Component) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventApplicationSaveToDB))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventApplicationSaveToDB))

	db, err := s.db()
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&DBAppStream{}).Error; err != nil {
			return fmt.Errorf(app.T_("Table cleanup error: %w"), err)
		}

		rows := make([]DBAppStream, 0, len(pkgMap))
		for pkgName, comps := range pkgMap {
			rows = append(rows, DBAppStream{
				PkgName:    pkgName,
				Components: comps,
			})
		}

		batchSize := 1000
		for i := 0; i < len(rows); i += batchSize {
			end := i + batchSize
			if end > len(rows) {
				end = len(rows)
			}
			if err = tx.Create(rows[i:end]).Error; err != nil {
				return fmt.Errorf(app.T_("Batch insert error: %w"), err)
			}
		}
		return nil
	})
}

// GetByPkgName возвращает компоненты AppStream для одного пакета.
func (s *DBService) GetByPkgName(ctx context.Context, name string) ([]Component, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	var row DBAppStream
	if err = db.WithContext(ctx).Where("pkgname = ?", name).First(&row).Error; err != nil {
		return nil, err
	}
	return row.Components, nil
}

// GetByPkgNames возвращает map[pkgName][]Component для списка имён пакетов.
func (s *DBService) GetByPkgNames(ctx context.Context, names []string) (map[string][]Component, error) {
	if len(names) == 0 {
		return nil, nil
	}

	db, err := s.db()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]Component, len(names))
	const chunkSize = 500

	for i := 0; i < len(names); i += chunkSize {
		end := i + chunkSize
		if end > len(names) {
			end = len(names)
		}
		var rows []DBAppStream
		if err = db.WithContext(ctx).Where("pkgname IN ?", names[i:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			result[r.PkgName] = r.Components
		}
	}

	return result, nil
}

// QueryComponents запрашивает компоненты с фильтрами, сортировкой и пагинацией.
func (s *DBService) QueryComponents(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit,
	offset int) ([]DBAppStream, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	query := db.WithContext(ctx).Model(&DBAppStream{})
	query = FilterApplier.Apply(query, filters)

	if sortField != "" {
		if err = FilterConfig.ValidateSortField(sortField); err != nil {
			return nil, err
		}
		order := strings.ToUpper(sortOrder)
		if order != "ASC" && order != "DESC" {
			order = "ASC"
		}
		query = query.Order(fmt.Sprintf("%s %s", sortField, order))
	}

	if limit > 0 {
		query = query.Limit(limit)
		if offset > 0 {
			query = query.Offset(offset)
		}
	}

	var rows []DBAppStream
	if err = query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
	}
	return rows, nil
}

// CountComponents возвращает количество записей с учётом фильтров.
func (s *DBService) CountComponents(ctx context.Context, filters []filter.Filter) (int64, error) {
	db, err := s.db()
	if err != nil {
		return 0, err
	}

	query := db.WithContext(ctx).Model(&DBAppStream{})
	query = FilterApplier.Apply(query, filters)

	var count int64
	if err = query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// DatabaseExist проверяет существование таблицы и наличие записей.
func (s *DBService) DatabaseExist(ctx context.Context) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	var count int64
	if err = db.WithContext(ctx).Model(&DBAppStream{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf(app.T_("Table %s exists but contains no records"), DBAppStream{}.TableName())
	}
	return nil
}

// GetCategories возвращает список уникальных категорий из всех компонентов.
func (s *DBService) GetCategories(ctx context.Context) ([]string, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	var categories []string
	err = db.WithContext(ctx).Raw(`
		SELECT DISTINCT kv.value
		FROM host_appstream_components ac,
		json_each(ac.components) AS comp,
		json_each(json_extract(comp.value, '$.categories')) AS kv
		ORDER BY kv.value
	`).Scan(&categories).Error
	if err != nil {
		return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
	}
	return categories, nil
}

// appStreamComponentApplier обрабатывает фильтры по полям components через json_extract.
func appStreamComponentApplier(query *gorm.DB, f filter.Filter) (*gorm.DB, bool) {
	if !strings.HasPrefix(f.Field, "components.") {
		return query, false
	}

	subField := strings.TrimPrefix(f.Field, "components.")
	if !filter.IsSafeFieldName(subField) {
		return query, false
	}

	if subField == "keywords" || subField == "categories" {
		colExpr, sqlOp, sqlValue := filter.ColOpToSQL("kv.value", f.Op, f.Value)
		return query.Where(
			fmt.Sprintf(`EXISTS (SELECT 1 FROM json_each(components) AS comp, json_each(json_extract(comp.value, '$.%s')) AS kv WHERE %s %s)`, subField, colExpr, sqlOp),
			sqlValue,
		), true
	}

	if subField == "name" || subField == "summary" || subField == "description" {
		colExpr, sqlOp, sqlValue := filter.ColOpToSQL("lv.value", f.Op, f.Value)
		return query.Where(
			fmt.Sprintf(`EXISTS (SELECT 1 FROM json_each(components) AS comp, json_each(json_extract(comp.value, '$.%s')) AS lv WHERE %s %s)`, subField, colExpr, sqlOp),
			sqlValue,
		), true
	}

	colExpr, sqlOp, sqlValue := filter.ColOpToSQL(
		fmt.Sprintf("json_extract(comp.value, '$.%s')", subField), f.Op, f.Value,
	)
	return query.Where(
		fmt.Sprintf(`EXISTS (SELECT 1 FROM json_each(components) AS comp WHERE %s %s)`, colExpr, sqlOp),
		sqlValue,
	), true
}

// ComponentFields определения полей фильтрации для Component (без префикса).
var ComponentFields = map[string]filter.FieldConfig{
	"type":            {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "STRING", "description": app.T_("Component type")}},
	"id":              {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": app.T_("Component ID")}},
	"pkgname":         {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": app.T_("Package name")}},
	"name":            {DefaultOp: filter.OpLike, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": app.T_("Application name")}},
	"summary":         {DefaultOp: filter.OpLike, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": app.T_("Short description")}},
	"project_license": {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": app.T_("Project license")}},
	"categories":      {DefaultOp: filter.OpContains, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpContains}, Extra: map[string]any{"type": "ARRAY", "description": app.T_("Categories")}},
	"keywords":        {DefaultOp: filter.OpContains, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpContains}, Extra: map[string]any{"type": "ARRAY", "description": app.T_("Keywords for search")}},
}

// PrefixedFields возвращает ComponentFields с добавленным префиксом к каждому ключу.
func PrefixedFields(prefix string) map[string]filter.FieldConfig {
	result := make(map[string]filter.FieldConfig, len(ComponentFields))
	for k, v := range ComponentFields {
		result[prefix+k] = v
	}
	return result
}

// PrefixedAppliers возвращает CustomAppliers с добавленным префиксом к каждому ключу
func PrefixedAppliers(prefix string, applier filter.FieldApplier) map[string]filter.FieldApplier {
	result := make(map[string]filter.FieldApplier, len(ComponentFields))
	for k := range ComponentFields {
		result[prefix+k] = applier
	}
	return result
}

// FilterConfig конфигурация фильтрации для AppStream компонентов.
var FilterConfig = &filter.Config{
	Fields: func() map[string]filter.FieldConfig {
		fields := PrefixedFields("components.")
		fields["pkgname"] = filter.FieldConfig{DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING"}}
		return fields
	}(),
}

// FilterApplier применяет фильтры к GORM-запросу для AppStream.
var FilterApplier = &filter.GormApplier{
	CustomAppliers: PrefixedAppliers("components.", appStreamComponentApplier),
}
