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
	"apm/internal/common/app"
	"apm/internal/common/filter"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"errors"
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

type DBDistroPackage struct {
	Container   string `gorm:"column:container;primaryKey"`
	Name        string `gorm:"column:name;primaryKey"`
	Version     string `gorm:"column:version"`
	Description string `gorm:"column:description"`
	Installed   bool   `gorm:"column:installed"`
	Exporting   bool   `gorm:"column:exporting"`
	Manager     string `gorm:"column:manager"`
}

type DistroDBService struct {
	dbManager app.DatabaseManager
	realDb    *gorm.DB
}

var initDistroDBMutex sync.Mutex

func (s *DistroDBService) db() (*gorm.DB, error) {
	initDistroDBMutex.Lock()
	defer initDistroDBMutex.Unlock()

	if s.realDb == nil {
		gormLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel: logger.Silent,
			},
		)

		conn, err := s.dbManager.GetUserDB()
		if err != nil {
			return nil, fmt.Errorf(app.T_("failed to get user DB: %w"), err)
		}

		s.realDb, err = gorm.Open(sqlite.Dialector{
			Conn:       conn,
			DriverName: "sqlite3",
		}, &gorm.Config{
			Logger: gormLogger,
		})
		if err != nil {
			return nil, fmt.Errorf("error opening GORM with existing db: %w", err)
		}

		if err = s.realDb.AutoMigrate(&DBDistroPackage{}); err != nil {
			return nil, fmt.Errorf("autoMigrate failed: %w", err)
		}
	}

	return s.realDb, nil
}

// NewDistroDBService создаёт новый сервис для работы с базой данных distrobox.
func NewDistroDBService(dbManager app.DatabaseManager) *DistroDBService {
	return &DistroDBService{
		dbManager: dbManager,
	}
}

// TableName задаёт имя таблицы.
func (DBDistroPackage) TableName() string {
	return "distrobox_packages"
}

// Преобразование GORM-модели -> бизнес-структура
func (dbp DBDistroPackage) fromDBModel() PackageInfo {
	return PackageInfo{
		Container:   dbp.Container,
		Name:        dbp.Name,
		Version:     dbp.Version,
		Description: dbp.Description,
		Installed:   dbp.Installed,
		Exporting:   dbp.Exporting,
		Manager:     dbp.Manager,
	}
}

// Преобразование бизнес-структуры -> GORM-модель
func (p PackageInfo) toDBModel() DBDistroPackage {
	return DBDistroPackage{
		Container:   p.Container,
		Name:        p.Name,
		Version:     p.Version,
		Description: p.Description,
		Installed:   p.Installed,
		Exporting:   p.Exporting,
		Manager:     p.Manager,
	}
}

// SavePackagesToDB сохраняет список пакетов (для конкретного containerName).
// Сначала удаляет старые записи (WHERE container=...), затем добавляет новые.
func (s *DistroDBService) SavePackagesToDB(ctx context.Context, containerName string, packages []PackageInfo) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroSavePackagesToDB))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroSavePackagesToDB))

	if len(containerName) == 0 {
		return errors.New(app.T_("The 'container' field cannot be empty when saving packages to the database"))
	}

	db, err := s.db()
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("container = ?", containerName).Delete(&DBDistroPackage{}).Error; err != nil {
			return err
		}

		batchSize := 1000
		for i := 0; i < len(packages); i += batchSize {
			end := i + batchSize
			if end > len(packages) {
				end = len(packages)
			}
			batch := packages[i:end]

			var dbEntries []DBDistroPackage
			for _, p := range batch {
				p.Container = containerName
				dbEntries = append(dbEntries, p.toDBModel())
			}

			if err := tx.Create(&dbEntries).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// DatabaseExist проверяет, есть ли вообще записи в таблице (не пустая ли).
func (s *DistroDBService) DatabaseExist(ctx context.Context) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	var count int64
	if err = db.WithContext(ctx).Model(&DBDistroPackage{}).Count(&count).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return errors.New(app.T_("The database does not have any records, it is necessary to create or update any container"))
		}
		return err
	}

	if count == 0 {
		return errors.New(app.T_("The database contains no records, you need to create or update any container"))
	}
	return nil
}

// ContainerDatabaseExist проверяет, есть ли у данного containerName вообще записи в таблице.
func (s *DistroDBService) ContainerDatabaseExist(ctx context.Context, containerName string) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	var count int64
	if err := db.WithContext(ctx).
		Model(&DBDistroPackage{}).
		Where("container = ?", containerName).
		Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf(app.T_("No records found for container %s"), containerName)
	}
	return nil
}

// CountTotalPackages возвращает количество записей (COUNT(*)) c учётом фильтра containerName и других полей.
func (s *DistroDBService) CountTotalPackages(containerName string, filters []filter.Filter) (int, error) {
	gormDB, err := s.db()
	if err != nil {
		return 0, err
	}

	db := gormDB.Model(&DBDistroPackage{})
	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}

	db = DistroFilterApplier.Apply(db, filters)

	var total int64
	if err = db.Count(&total).Error; err != nil {
		return 0, err
	}
	return int(total), nil
}

// QueryPackages возвращает записи с фильтрами, сортировкой, limit/offset.
func (s *DistroDBService) QueryPackages(containerName string, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]PackageInfo, error) {
	gormDB, err := s.db()
	if err != nil {
		return nil, err
	}

	db := gormDB.Model(&DBDistroPackage{})

	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}

	db = DistroFilterApplier.Apply(db, filters)

	if sortField != "" {
		if err = DistroFilterConfig.ValidateSortField(sortField); err != nil {
			return nil, err
		}
		upperOrder := strings.ToUpper(sortOrder)
		if upperOrder != "ASC" && upperOrder != "DESC" {
			upperOrder = "ASC"
		}
		db = db.Order(fmt.Sprintf("%s %s", sortField, upperOrder))
	}

	if limit > 0 {
		db = db.Limit(limit)
		if offset > 0 {
			db = db.Offset(offset)
		}
	}

	var dbPackages []DBDistroPackage
	if err = db.Find(&dbPackages).Error; err != nil {
		return nil, err
	}

	packages := make([]PackageInfo, 0, len(dbPackages))
	for _, dbp := range dbPackages {
		packages = append(packages, dbp.fromDBModel())
	}
	return packages, nil
}

// FindPackagesByName ищет пакеты по name (LIKE) + container (при необходимости).
func (s *DistroDBService) FindPackagesByName(containerName, partialName string) ([]PackageInfo, error) {
	gormDB, err := s.db()
	if err != nil {
		return nil, err
	}

	db := gormDB.Model(&DBDistroPackage{})

	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}
	if partialName != "" {
		db = db.Where("name LIKE ?", "%"+partialName+"%")
	}

	var dbPackages []DBDistroPackage
	if err = db.Find(&dbPackages).Error; err != nil {
		return nil, err
	}

	packages := make([]PackageInfo, 0, len(dbPackages))
	for _, dbp := range dbPackages {
		packages = append(packages, dbp.fromDBModel())
	}
	return packages, nil
}

// UpdatePackageField обновляет (installed или exporting) для конкретного container+name.
func (s *DistroDBService) UpdatePackageField(ctx context.Context, containerName, name, fieldName string, value bool) {
	allowedFields := map[string]bool{
		"installed": true,
		"exporting": true,
	}

	if !allowedFields[fieldName] {
		app.Log.Errorf(app.T_("The field %s cannot be updated."), fieldName)
		return
	}

	updateMap := map[string]interface{}{
		fieldName: value,
	}

	db, err := s.db()
	if err != nil {
		app.Log.Error(err)
		return
	}

	if err = db.WithContext(ctx).
		Model(&DBDistroPackage{}).
		Where("container = ? AND name = ?", containerName, name).
		Updates(updateMap).Error; err != nil {
		app.Log.Error(err)
	}
}

// GetPackageInfoByName возвращает запись (container+name) из таблицы.
func (s *DistroDBService) GetPackageInfoByName(containerName, name string) (PackageInfo, error) {
	db, err := s.db()
	if err != nil {
		return PackageInfo{}, err
	}

	var dbp DBDistroPackage
	if err = db.
		Where("container = ? AND name = ?", containerName, name).
		First(&dbp).Error; err != nil {
		return PackageInfo{}, err
	}
	return dbp.fromDBModel(), nil
}

// DeletePackagesFromContainer удаляет все записи для указанного контейнера.
func (s *DistroDBService) DeletePackagesFromContainer(ctx context.Context, containerName string) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	if err = db.WithContext(ctx).
		Where("container = ?", containerName).
		Delete(&DBDistroPackage{}).Error; err != nil {
		return fmt.Errorf(app.T_("Error deleting container records %s: %v"), containerName, err)
	}
	return nil
}

// distroBoolApplier обрабатывает булевые фильтры installed и exporting.
func distroBoolApplier(query *gorm.DB, f filter.Filter) (*gorm.DB, bool) {
	boolVal, ok := helper.ParseBool(f.Value)
	if !ok {
		return query, true
	}
	return query.Where(clause.Eq{Column: clause.Column{Name: f.Field}, Value: boolVal}), true
}

// DistroFilterConfig конфигурация фильтрации для distrobox пакетов.
var DistroFilterConfig = &filter.Config{
	Fields: map[string]filter.FieldConfig{
		"name":        {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package name"}},
		"version":     {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package version"}},
		"description": {DefaultOp: filter.OpLike, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Package description"}},
		"container":   {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe, filter.OpLike}, Extra: map[string]any{"type": "STRING", "description": "Container name"}},
		"installed":   {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "BOOL", "description": "Installation status"}},
		"exporting":   {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "BOOL", "description": "Export status"}},
		"manager":     {DefaultOp: filter.OpEq, Sortable: true, AllowedOps: []filter.Op{filter.OpEq, filter.OpNe}, Extra: map[string]any{"type": "STRING", "description": "Package manager", "choice": []string{"apt-get", "apt", "pacman"}}},
	},
}

// DistroFilterApplier применяет фильтры к GORM-запросу для distrobox пакетов.
var DistroFilterApplier = &filter.GormApplier{
	CustomAppliers: map[string]filter.FieldApplier{
		"installed": distroBoolApplier,
		"exporting": distroBoolApplier,
	},
}
