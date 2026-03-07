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
	"apm/internal/common/filter"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	"context"
	"database/sql/driver"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"gorm.io/gorm/clause"

	"gorm.io/gorm/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// PackageDBService предоставляет сервис для операций с базой данных пакетов.
type PackageDBService struct {
	dbManager app.DatabaseManager
	realDb    *gorm.DB
}

var initDBMutex sync.Mutex

// syncDBMutex защищает операции синхронизации базы пакетов.
var syncDBMutex sync.Mutex

func (s *PackageDBService) db() (*gorm.DB, error) {
	initDBMutex.Lock()
	defer initDBMutex.Unlock()

	if s.realDb == nil {
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel: logger.Silent,
			},
		)

		conn, err := s.dbManager.GetSystemDB()
		if err != nil {
			return nil, fmt.Errorf(app.T_("failed to get system DB: %w"), err)
		}
		s.realDb, err = gorm.Open(sqlite.Dialector{
			Conn:       conn,
			DriverName: "sqlite3",
		}, &gorm.Config{
			Logger: gormLogger,
		})

		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к SQLite через GORM: %w", err)
		}

		// Автоматическая миграция
		if err = s.realDb.AutoMigrate(&DBPackage{}); err != nil {
			return nil, fmt.Errorf("ошибка миграции структуры таблицы: %w", err)
		}
	}

	return s.realDb, nil
}

// NewPackageDBService создаёт новый сервис для работы с базой данных пакетов.
func NewPackageDBService(dbManager app.DatabaseManager) *PackageDBService {
	return &PackageDBService{
		dbManager: dbManager,
	}
}

type PackageType uint8

const (
	PackageTypeSystem PackageType = iota
	PackageTypeStplr
)

func (t PackageType) String() string {
	switch t {
	case PackageTypeSystem:
		return "system"
	case PackageTypeStplr:
		return "stplr"
	default:
		return "unknown"
	}
}

func (t PackageType) Value() (driver.Value, error) { return int64(t), nil }

// DBPackage описывает модель пакета для GORM.
type DBPackage struct {
	Name             string      `gorm:"column:name;primaryKey"`
	Architecture     string      `gorm:"column:architecture"`
	Section          string      `gorm:"column:section"`
	InstalledSize    int         `gorm:"column:installedSize"`
	Maintainer       string      `gorm:"column:maintainer"`
	Version          string      `gorm:"column:version;primaryKey"`
	VersionRaw       string      `gorm:"column:versionRaw"`
	VersionInstalled string      `gorm:"column:versionInstalled"`
	Depends          string      `gorm:"column:depends"`
	Aliases          string      `gorm:"column:aliases"`
	Provides         string      `gorm:"column:provides"`
	Size             int         `gorm:"column:size"`
	Filename         string      `gorm:"column:filename"`
	Summary          string      `gorm:"column:summary"`
	Description      string      `gorm:"column:description"`
	IDAppStream      *uint       `gorm:"column:idAppStream"`
	Changelog        string      `gorm:"column:changelog"`
	Installed        bool        `gorm:"column:installed"`
	TypePackage      PackageType `gorm:"column:typePackage"`
	Files            string      `gorm:"column:files"`
}

// TableName задаёт имя таблицы.
func (DBPackage) TableName() string {
	return "host_image_packages"
}

// fromDBModel преобразует модель базы данных в бизнес-структуру.
func (dbp DBPackage) fromDBModel() Package {
	p := Package{
		Name:             dbp.Name,
		Architecture:     dbp.Architecture,
		Section:          dbp.Section,
		InstalledSize:    dbp.InstalledSize,
		Maintainer:       dbp.Maintainer,
		Version:          dbp.Version,
		VersionRaw:       dbp.VersionRaw,
		VersionInstalled: dbp.VersionInstalled,
		Size:             dbp.Size,
		Filename:         dbp.Filename,
		Summary:          dbp.Summary,
		Description:      dbp.Description,
		Changelog:        dbp.Changelog,
		Installed:        dbp.Installed,
		TypePackage:      int(dbp.TypePackage),
		HasAppStream:     dbp.IDAppStream != nil,
	}
	if strings.TrimSpace(dbp.Aliases) != "" {
		p.Aliases = strings.Split(dbp.Aliases, ",")
	}
	if strings.TrimSpace(dbp.Depends) != "" {
		p.Depends = strings.Split(dbp.Depends, ",")
	}
	if strings.TrimSpace(dbp.Provides) != "" {
		p.Provides = strings.Split(dbp.Provides, ",")
	}
	if strings.TrimSpace(dbp.Files) != "" {
		p.Files = strings.Split(dbp.Files, ",")
	}
	return p
}

// toDBModel преобразует бизнес-структуру в модель базы данных.
func (p Package) toDBModel() DBPackage {
	dbp := DBPackage{
		Name:             p.Name,
		Architecture:     p.Architecture,
		Section:          p.Section,
		InstalledSize:    p.InstalledSize,
		Maintainer:       p.Maintainer,
		Version:          p.Version,
		VersionRaw:       p.VersionRaw,
		VersionInstalled: p.VersionInstalled,
		Size:             p.Size,
		Filename:         p.Filename,
		Summary:          p.Summary,
		Description:      p.Description,
		Changelog:        p.Changelog,
		Installed:        p.Installed,
		TypePackage:      PackageType(p.TypePackage),
	}
	if len(p.Aliases) > 0 {
		dbp.Aliases = strings.Join(p.Aliases, ",")
	}
	if len(p.Depends) > 0 {
		dbp.Depends = strings.Join(p.Depends, ",")
	}
	if len(p.Provides) > 0 {
		dbp.Provides = strings.Join(p.Provides, ",")
	}
	if len(p.Files) > 0 {
		dbp.Files = strings.Join(p.Files, ",")
	}
	return dbp
}

// SavePackagesToDB сохраняет список пакетов (перезапись всей таблицы).
func (s *PackageDBService) SavePackagesToDB(ctx context.Context, packages []Package) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()

	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemSavePackagesToDB))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemSavePackagesToDB))

	db, err := s.db()
	if err != nil {
		return err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// Очищаем таблицу
		if errDel := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&DBPackage{}).Error; errDel != nil {
			return fmt.Errorf(app.T_("Table cleanup error: %w"), errDel)
		}

		batchSize := 1000
		n := len(packages)
		for i := 0; i < n; i += batchSize {
			end := i + batchSize
			if end > n {
				end = n
			}
			batch := packages[i:end]

			// Конвертация в список DBPackage
			var dbPackages []DBPackage
			for _, pkg := range batch {
				dbPackages = append(dbPackages, pkg.toDBModel())
			}

			if errCreate := tx.Create(&dbPackages).Error; errCreate != nil {
				return fmt.Errorf(app.T_("Batch insert error: %w"), errCreate)
			}
		}
		return nil
	})

	return err
}

// GetPackageByName возвращает запись пакета по имени.
func (s *PackageDBService) GetPackageByName(ctx context.Context, packageName string) (Package, error) {
	db, err := s.db()
	if err != nil {
		return Package{}, err
	}

	var dbPkg DBPackage
	err = db.WithContext(ctx).
		Where("name = ? OR (',' || aliases || ',') LIKE ? OR (',' || files || ',') LIKE ?",
			packageName, "%,"+packageName+",%", "%,"+packageName+",%").
		First(&dbPkg).Error
	if err != nil {
		return Package{}, fmt.Errorf(app.T_("Failed to get information about the package %s"), packageName)
	}

	return dbPkg.fromDBModel(), nil
}

// GetPackagesByNames возвращает пакеты по списку имён одним batch-запросом
func (s *PackageDBService) GetPackagesByNames(ctx context.Context, names []string) ([]Package, error) {
	if len(names) == 0 {
		return nil, nil
	}

	db, err := s.db()
	if err != nil {
		return nil, err
	}

	const chunkSize = 500
	var DBPackages []DBPackage

	for i := 0; i < len(names); i += chunkSize {
		end := i + chunkSize
		if end > len(names) {
			end = len(names)
		}
		var chunk []DBPackage
		if err = db.WithContext(ctx).Where("name IN ?", names[i:end]).Find(&chunk).Error; err != nil {
			return nil, err
		}
		DBPackages = append(DBPackages, chunk...)
	}

	foundNames := make(map[string]bool, len(DBPackages))
	for _, p := range DBPackages {
		foundNames[p.Name] = true
	}

	var missingNames []string
	for _, name := range names {
		if !foundNames[name] {
			missingNames = append(missingNames, name)
		}
	}

	for _, name := range missingNames {
		var dbPkg DBPackage
		err = db.WithContext(ctx).
			Where("(',' || aliases || ',') LIKE ? OR (',' || files || ',') LIKE ?",
				"%,"+name+",%", "%,"+name+",%").
			First(&dbPkg).Error
		if err != nil {
			continue
		}
		DBPackages = append(DBPackages, dbPkg)
	}

	result := make([]Package, 0, len(DBPackages))
	for _, p := range DBPackages {
		result = append(result, p.fromDBModel())
	}
	return result, nil
}

// SyncPackageInstallationInfo синхронизирует базу пакетов
func (s *PackageDBService) SyncPackageInstallationInfo(ctx context.Context, installedPackages map[string]string) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()

	db, err := s.db()
	if err != nil {
		return err
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err = tx.Exec("DROP TABLE IF EXISTS tmp_installed").Error; err != nil {
			return fmt.Errorf(app.T_("Temporary table drop error: %w"), err)
		}

		if err = tx.Exec("CREATE TEMPORARY TABLE tmp_installed (name TEXT PRIMARY KEY, version TEXT)").Error; err != nil {
			return fmt.Errorf(app.T_("Temporary table creation error: %w"), err)
		}

		var rows []map[string]interface{}
		for name, version := range installedPackages {
			rows = append(rows, map[string]interface{}{
				"name":    name,
				"version": version,
			})
		}
		if len(rows) > 0 {
			if err = tx.Table("tmp_installed").Create(rows).Error; err != nil {
				return fmt.Errorf(app.T_("Batch insert into temporary table error: %w"), err)
			}
		}

		updateSQL := `
			UPDATE host_image_packages
			SET
				installed = CASE
					WHEN EXISTS (
						SELECT 1 FROM tmp_installed t WHERE t.name = host_image_packages.name
					) THEN 1
					ELSE 0
				END,
				versionInstalled = COALESCE(
					(SELECT t.version FROM tmp_installed t WHERE t.name = host_image_packages.name),
					''
				)
		`
		if err = tx.Exec(updateSQL).Error; err != nil {
			return fmt.Errorf(app.T_("Batch update error: %w"), err)
		}

		return nil
	})
	return err
}

// SearchPackagesByNameLike ищет пакеты по произвольному шаблону LIKE
func (s *PackageDBService) SearchPackagesByNameLike(ctx context.Context, likePattern string, installed bool) ([]Package, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	query := db.WithContext(ctx).Model(&DBPackage{}).
		Where("name LIKE ?", likePattern)

	if installed {
		query = query.Where("installed = ?", true)
	}

	var dbPkgs []DBPackage
	if err = query.Find(&dbPkgs).Error; err != nil {
		return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
	}

	// Fallback: поиск по файлам если по имени ничего не нашли
	if len(dbPkgs) == 0 {
		query = db.WithContext(ctx).Model(&DBPackage{}).
			Where("files LIKE ?", likePattern)
		if installed {
			query = query.Where("installed = ?", true)
		}
		if err = query.Find(&dbPkgs).Error; err != nil {
			return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
		}
	}

	result := make([]Package, 0, len(dbPkgs))
	for _, dbp := range dbPkgs {
		result = append(result, dbp.fromDBModel())
	}
	return result, nil
}

// SearchPackagesMultiLimit ищет пакеты по произвольному шаблону LIKE для автодополнения
func (s *PackageDBService) SearchPackagesMultiLimit(ctx context.Context, likePattern string, limit int, installed bool) ([]Package, error) {
	if limit <= 0 {
		limit = 100
	}

	db, err := s.db()
	if err != nil {
		return nil, err
	}

	query := db.WithContext(ctx).
		Model(&DBPackage{}).
		Where("name LIKE ?", likePattern).
		Limit(limit)

	if installed {
		query = query.Where("installed = ?", true)
	}

	var dbPkgs []DBPackage
	if err = query.Find(&dbPkgs).Error; err != nil {
		return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
	}

	// Fallback: поиск по файлам если по имени ничего не нашли
	if len(dbPkgs) == 0 {
		query = db.WithContext(ctx).Model(&DBPackage{}).
			Where("files LIKE ?", likePattern).
			Limit(limit)
		if installed {
			query = query.Where("installed = ?", true)
		}
		if err = query.Find(&dbPkgs).Error; err != nil {
			return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
		}
	}

	res := make([]Package, 0, len(dbPkgs))
	for _, dbp := range dbPkgs {
		res = append(res, dbp.fromDBModel())
	}
	return res, nil
}

// QueryHostImagePackages возвращает пакеты с возможностью фильтрации и сортировки
func (s *PackageDBService) QueryHostImagePackages(
	ctx context.Context,
	filters []filter.Filter,
	sortField, sortOrder string,
	limit, offset int,
) ([]Package, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	query := db.WithContext(ctx).Model(&DBPackage{})

	// Применяем фильтры
	query = SystemFilterApplier.Apply(query, filters)

	if sortField != "" {
		if err = SystemFilterConfig.ValidateSortField(sortField); err != nil {
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

	var dbPkgs []DBPackage
	if err = query.Find(&dbPkgs).Error; err != nil {
		return nil, fmt.Errorf(app.T_("Query execution error: %w"), err)
	}

	// Преобразование к бизнес-структурам
	var result []Package
	for _, dbp := range dbPkgs {
		result = append(result, dbp.fromDBModel())
	}

	return result, nil
}

// CountHostImagePackages возвращает количество записей с учётом фильтров
func (s *PackageDBService) CountHostImagePackages(ctx context.Context, filters []filter.Filter) (int64, error) {
	db, err := s.db()
	if err != nil {
		return 0, err
	}

	query := db.WithContext(ctx).Model(&DBPackage{})
	query = SystemFilterApplier.Apply(query, filters)

	var totalCount int64
	if err = query.Count(&totalCount).Error; err != nil {
		return 0, fmt.Errorf(app.T_("Package count error: %w"), err)
	}

	return totalCount, nil
}

// appStreamApplier обрабатывает фильтры по полям appStream через JOIN на host_appstream_components.
func appStreamApplier(query *gorm.DB, f filter.Filter) (*gorm.DB, bool) {
	if !strings.HasPrefix(f.Field, swcat.AppStreamPrefix) {
		return query, false
	}

	subField := strings.TrimPrefix(f.Field, swcat.AppStreamPrefix)
	if !filter.IsSafeFieldName(subField) {
		return query, false
	}

	// Сразу отсекаем пакеты без AppStream
	query = query.Where("idAppStream IS NOT NULL")

	if subField == "pkgname" {
		colExpr, sqlOp, sqlValue := filter.ColOpToSQL("ac.pkgname", f.Op, f.Value)
		return query.Where(
			fmt.Sprintf(`EXISTS (
				SELECT 1 FROM host_appstream_components ac
				WHERE ac.id = host_image_packages.idAppStream AND %s %s
			)`, colExpr, sqlOp),
			sqlValue,
		), true
	}

	if subField == "keywords" || subField == "categories" {
		colExpr, sqlOp, sqlValue := filter.ColOpToSQL("kv.value", f.Op, f.Value)
		return query.Where(
			fmt.Sprintf(`EXISTS (
				SELECT 1 FROM host_appstream_components ac,
				json_each(ac.components) AS comp,
				json_each(json_extract(comp.value, '$.%s')) AS kv
				WHERE ac.id = host_image_packages.idAppStream AND %s %s
			)`, subField, colExpr, sqlOp),
			sqlValue,
		), true
	}

	if subField == "name" || subField == "summary" || subField == "description" {
		colExpr, sqlOp, sqlValue := filter.ColOpToSQL("lv.value", f.Op, f.Value)
		return query.Where(
			fmt.Sprintf(`EXISTS (
				SELECT 1 FROM host_appstream_components ac,
				json_each(ac.components) AS comp,
				json_each(json_extract(comp.value, '$.%s')) AS lv
				WHERE ac.id = host_image_packages.idAppStream AND %s %s
			)`, subField, colExpr, sqlOp),
			sqlValue,
		), true
	}

	colExpr, sqlOp, sqlValue := filter.ColOpToSQL(
		fmt.Sprintf("json_extract(comp.value, '$.%s')", subField), f.Op, f.Value,
	)
	return query.Where(
		fmt.Sprintf(`EXISTS (
			SELECT 1 FROM host_appstream_components ac,
			json_each(ac.components) AS comp
			WHERE ac.id = host_image_packages.idAppStream AND %s %s
		)`, colExpr, sqlOp),
		sqlValue,
	), true
}

// isAppApplier обрабатывает специальный фильтр isApp.
func isAppApplier(query *gorm.DB, f filter.Filter) (*gorm.DB, bool) {
	boolVal, ok := helper.ParseBool(f.Value)
	if !ok {
		return query, true
	}
	if boolVal {
		return query.Where("idAppStream IS NOT NULL"), true
	}
	return query.Where("idAppStream IS NULL"), true
}

// installedApplier обрабатывает фильтр installed с ParseBool.
func installedApplier(query *gorm.DB, f filter.Filter) (*gorm.DB, bool) {
	boolVal, ok := helper.ParseBool(f.Value)
	if !ok {
		return query, true
	}
	return query.Where(clause.Eq{Column: clause.Column{Name: "installed"}, Value: boolVal}), true
}

// SaveSinglePackage сохраняет один пакет в базу данных без очистки таблицы
func (s *PackageDBService) SaveSinglePackage(ctx context.Context, pkg Package) error {
	dbPkg := pkg.toDBModel()

	db, err := s.db()
	if err != nil {
		return err
	}

	// Используем OnConflict для UPSERT логики
	// Primary key состоит из name + version, используем оба поля
	err = db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}, {Name: "version"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"architecture", "section", "installed_size", "maintainer",
				"versionRaw", "versionInstalled", "depends", "provides",
				"size", "filename", "summary", "description", "changelog",
				"installed", "typePackage", "aliases", "files",
			}),
		}).
		Create(&dbPkg).Error

	if err != nil {
		return fmt.Errorf("failed to save package %s: %w", pkg.Name, err)
	}

	return nil
}

// PackageDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func (s *PackageDBService) PackageDatabaseExist(ctx context.Context) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	var count int64
	if err = db.WithContext(ctx).
		Model(&DBPackage{}).
		Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf(app.T_("Table %s exists but contains no records"), DBPackage{}.TableName())
	}

	return nil
}

// UpdateAppStreamLinks обновляет idAppStream
func (s *PackageDBService) UpdateAppStreamLinks(ctx context.Context) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Exec(`
		UPDATE host_image_packages
		SET idAppStream = (
			SELECT id FROM host_appstream_components
			WHERE host_appstream_components.pkgname = host_image_packages.name
			LIMIT 1
		)
	`).Error
}

// GetSections возвращает список уникальных секций пакетов.
func (s *PackageDBService) GetSections(ctx context.Context) ([]string, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	var sections []string
	err = db.WithContext(ctx).Model(&DBPackage{}).
		Distinct("section").
		Where("section != ''").
		Order("section").
		Pluck("section", &sections).Error
	if err != nil {
		return nil, err
	}
	return sections, nil
}

// SystemFilterConfig конфигурация фильтрации для системных пакетов.
var SystemFilterConfig = &filter.Config{
	Fields: func() map[string]filter.FieldConfig {
		fields := map[string]filter.FieldConfig{
			"name":             {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package name"}},
			"isApp":            {DefaultOp: filter.OpEq, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "BOOL", "description": "Has AppStream data (is application)"}},
			"section":          {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package section (e.g. Shells, Editors)"}},
			"installedSize":    {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpGt, filter.OpGte, filter.OpLt, filter.OpLte}, Extra: map[string]any{"type": "INTEGER", "description": "Installed size in bytes"}},
			"maintainer":       {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package maintainer"}},
			"version":          {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Available version"}},
			"versionRaw":       {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Raw version string"}},
			"versionInstalled": {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Installed version"}},
			"depends":          {DefaultOp: filter.OpContains, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike, filter.OpContains}, Extra: map[string]any{"type": "STRING", "description": "Package dependencies"}},
			"provides":         {DefaultOp: filter.OpContains, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike, filter.OpContains}, Extra: map[string]any{"type": "STRING", "description": "Provided virtual packages"}},
			"size":             {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpGt, filter.OpGte, filter.OpLt, filter.OpLte}, Extra: map[string]any{"type": "INTEGER", "description": "Download size in bytes"}},
			"filename":         {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package filename"}},
			"description":      {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package description"}},
			"summary":          {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Short package summary"}},
			"changelog":        {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Last changelog entry"}},
			"installed":        {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "BOOL", "description": "Installation status"}},
			"typePackage": {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{
				"type":        "ENUM",
				"description": app.T_("Package type"),
				"info": map[PackageType]string{
					PackageTypeSystem: "System package",
				},
			}},
			"files": {DefaultOp: filter.OpContains, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike, filter.OpContains}, Extra: map[string]any{"type": "STRING", "description": "Package file list"}},
		}
		for k, v := range swcat.PrefixedFields(swcat.AppStreamPrefix) {
			fields[k] = v
		}
		return fields
	}(),
}

// SystemFilterApplier применяет фильтры к GORM-запросу для системных пакетов.
var SystemFilterApplier = &filter.GormApplier{
	CustomAppliers: func() map[string]filter.FieldApplier {
		appliers := swcat.PrefixedAppliers(swcat.AppStreamPrefix, appStreamApplier)
		appliers["isApp"] = isAppApplier
		appliers["installed"] = installedApplier
		return appliers
	}(),
}
