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
	"apm/internal/common/appstream"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
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

// PackageDBService — сервис для операций с базой данных пакетов
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

		var err error
		s.realDb, err = gorm.Open(sqlite.Dialector{
			Conn:       s.dbManager.GetSystemDB(),
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

// NewPackageDBService — конструктор сервиса
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

// DBPackage — модель для GORM, отражающая структуру таблицы в БД.
type DBPackage struct {
	Name             string               `gorm:"column:name;primaryKey"`
	Architecture     string               `gorm:"column:architecture"`
	Section          string               `gorm:"column:section"`
	InstalledSize    int                  `gorm:"column:installed_size"`
	Maintainer       string               `gorm:"column:maintainer"`
	Version          string               `gorm:"column:version;primaryKey"`
	VersionRaw       string               `gorm:"column:versionRaw"`
	VersionInstalled string               `gorm:"column:versionInstalled"`
	Depends          string               `gorm:"column:depends"`
	Aliases          string               `gorm:"column:aliases"`
	Provides         string               `gorm:"column:provides"`
	Size             int                  `gorm:"column:size"`
	Filename         string               `gorm:"column:filename"`
	Summary          string               `gorm:"column:summary"`
	Description      string               `gorm:"column:description"`
	AppStream        *appstream.Component `gorm:"column:appStream;serializer:json;type:TEXT"`
	Changelog        string               `gorm:"column:changelog"`
	Installed        bool                 `gorm:"column:installed"`
	TypePackage      PackageType          `gorm:"column:typePackage"`
}

// TableName — задаём имя таблицы
func (DBPackage) TableName() string {
	return "host_image_packages"
}

// fromDBModel — вспомогательная функция, преобразующая DBPackage (модель БД) в бизнес-структуру Package.
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
		AppStream:        dbp.AppStream,
		Changelog:        dbp.Changelog,
		Installed:        dbp.Installed,
		TypePackage:      int(dbp.TypePackage),
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
	return p
}

// toDBModel — обратная функция, преобразующая бизнес-структуру Package в DBPackage для сохранения в БД.
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
		AppStream:        p.AppStream,
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
		Where("name = ? OR (',' || aliases || ',') LIKE ?", packageName, "%,"+packageName+",%").
		First(&dbPkg).Error
	if err != nil {
		return Package{}, fmt.Errorf(app.T_("Failed to get information about the package %s"), packageName)
	}

	return dbPkg.fromDBModel(), nil
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

	res := make([]Package, 0, len(dbPkgs))
	for _, dbp := range dbPkgs {
		res = append(res, dbp.fromDBModel())
	}
	return res, nil
}

// QueryHostImagePackages возвращает пакеты с возможностью фильтрации и сортировки
func (s *PackageDBService) QueryHostImagePackages(
	ctx context.Context,
	filters map[string]interface{},
	sortField, sortOrder string,
	limit, offset int,
) ([]Package, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	query := db.WithContext(ctx).Model(&DBPackage{})

	// Применяем фильтры через общий метод
	query, err = s.applyFilters(query, filters)
	if err != nil {
		return nil, err
	}

	if sortField != "" {
		if !isAllowedField(sortField, allowedSortFields) {
			return nil, fmt.Errorf(app.T_("Invalid sort field: %s. Available fields: %s"), sortField, strings.Join(allowedSortFields, ", "))
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
func (s *PackageDBService) CountHostImagePackages(ctx context.Context, filters map[string]interface{}) (int64, error) {
	db, err := s.db()
	if err != nil {
		return 0, err
	}

	query := db.WithContext(ctx).Model(&DBPackage{})

	query, err = s.applyFilters(query, filters)
	if err != nil {
		return 0, err
	}

	var totalCount int64
	if err = query.Count(&totalCount).Error; err != nil {
		return 0, fmt.Errorf(app.T_("Package count error: %w"), err)
	}

	return totalCount, nil
}

// applyFilters применяет фильтры к запросу и возвращает модифицированный *gorm.DB.
// Если встречается недопустимое поле, возвращается ошибка.
func (s *PackageDBService) applyFilters(query *gorm.DB, filters map[string]interface{}) (*gorm.DB, error) {
	for field, value := range filters {
		if !isAllowedField(field, AllowedFilterFields) {
			return nil, fmt.Errorf(app.T_("Invalid filter field: %s. Available fields: %s"), field, strings.Join(AllowedFilterFields, ", "))
		}

		switch field {
		case "isApp":
			boolVal, ok := helper.ParseBool(value)
			if !ok {
				continue
			}
			if boolVal {
				query = query.Where("appStream IS NOT NULL AND appStream <> ''")
			} else {
				query = query.Where("appStream IS NULL OR appStream = ''")
			}
		case "installed":
			boolVal, ok := helper.ParseBool(value)
			if !ok {
				continue
			}
			query = query.Where("installed = ?", boolVal)
		case "typePackage":
			query = query.Where("typePackage = ?", value)
		case "depends":
			if strVal, ok := value.(string); ok {
				query = query.Where("(',' || depends || ',') LIKE ?", "%,"+strVal+",%")
			} else {
				query = query.Where("(',' || depends || ',') LIKE ?", fmt.Sprintf("%%,%v,%%", value))
			}
		case "provides":
			if strVal, ok := value.(string); ok {
				query = query.Where("(',' || provides || ',') LIKE ?", "%,"+strVal+",%")
			} else {
				query = query.Where("(',' || provides || ',') LIKE ?", fmt.Sprintf("%%,%v,%%", value))
			}
		default:
			if strVal, ok := value.(string); ok {
				query = query.Where(fmt.Sprintf("%s LIKE ?", field), "%"+strVal+"%")
			} else {
				query = query.Where(fmt.Sprintf("%s = ?", field), value)
			}
		}
	}
	return query, nil
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
				"size", "filename", "summary", "description", "appStream", "changelog",
				"installed", "typePackage", "aliases",
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

// Вспомогательная функция для проверки разрешённых полей
func isAllowedField(field string, allowed []string) bool {
	for _, f := range allowed {
		if f == field {
			return true
		}
	}
	return false
}

// Списки разрешённых полей для сортировки
var allowedSortFields = []string{
	"name",
	"section",
	"installedSize",
	"maintainer",
	"version",
	"versionRaw",
	"versionInstalled",
	"depends",
	"provides",
	"size",
	"filename",
	"description",
	"summary",
	"changelog",
	"installed",
	"typePackage",
}

// AllowedFilterFields Списки разрешённых полей для фильтрации.
var AllowedFilterFields = []string{
	"name",
	"isApp",
	"section",
	"installedSize",
	"maintainer",
	"version",
	"versionRaw",
	"versionInstalled",
	"depends",
	"provides",
	"size",
	"filename",
	"description",
	"summary",
	"changelog",
	"installed",
	"typePackage",
}
