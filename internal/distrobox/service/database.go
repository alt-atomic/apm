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
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"context"
	"database/sql"
	"fmt"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"slices"
	"strings"

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
	db *gorm.DB
}

// NewDistroDBService — конструктор сервиса
func NewDistroDBService(db *sql.DB) (*DistroDBService, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel: logger.Silent,
		},
	)

	gormDB, err := gorm.Open(sqlite.Dialector{
		Conn:       db,
		DriverName: "sqlite3",
	}, &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		return nil, fmt.Errorf("error opening GORM with existing db: %w", err)
	}

	// Автоматическая миграция
	if err = gormDB.AutoMigrate(&DBDistroPackage{}); err != nil {
		return nil, fmt.Errorf("autoMigrate failed: %w", err)
	}

	return &DistroDBService{
		db: gormDB,
	}, nil
}

// TableName - задаём имя таблицы
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
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.SavePackagesToDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.SavePackagesToDB"))

	if len(containerName) == 0 {
		return fmt.Errorf(lib.T_("The 'container' field cannot be empty when saving packages to the database"))
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("container = ?", containerName).Delete(&DBDistroPackage{}).Error; err != nil {
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
	var count int64
	if err := s.db.WithContext(ctx).Model(&DBDistroPackage{}).Count(&count).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf(lib.T_("The database does not have any records, it is necessary to create or update any container"))
		}
		return err
	}

	if count == 0 {
		return fmt.Errorf(lib.T_("The database contains no records, you need to create or update any container"))
	}
	return nil
}

// ContainerDatabaseExist проверяет, есть ли у данного containerName вообще записи в таблице.
func (s *DistroDBService) ContainerDatabaseExist(ctx context.Context, containerName string) error {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&DBDistroPackage{}).
		Where("container = ?", containerName).
		Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf(lib.T_("No records found for container %s"), containerName)
	}
	return nil
}

// CountTotalPackages возвращает количество записей (COUNT(*)) c учётом фильтра containerName и других полей.
func (s *DistroDBService) CountTotalPackages(containerName string, filters map[string]interface{}) (int, error) {
	db := s.db.Model(&DBDistroPackage{})
	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}

	db, err := s.applyFilters(db, filters)
	if err != nil {
		return 0, err
	}

	var total int64
	if err = db.Count(&total).Error; err != nil {
		return 0, err
	}
	return int(total), nil
}

// QueryPackages возвращает записи с фильтрами, сортировкой, limit/offset.
func (s *DistroDBService) QueryPackages(containerName string, filters map[string]interface{}, sortField, sortOrder string, limit, offset int) ([]PackageInfo, error) {
	db := s.db.Model(&DBDistroPackage{})

	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}

	var err error
	db, err = s.applyFilters(db, filters)
	if err != nil {
		return nil, err
	}

	if sortField != "" {
		if !s.isAllowedField(sortField, allowedSortFields) {
			return nil, fmt.Errorf(lib.T_("Invalid sort field: %s. Available fields: %s."), sortField, strings.Join(allowedSortFields, ", "))
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
	db := s.db.Model(&DBDistroPackage{})

	if containerName != "" {
		db = db.Where("container = ?", containerName)
	}
	if partialName != "" {
		db = db.Where("name LIKE ?", "%"+partialName+"%")
	}

	var dbPackages []DBDistroPackage
	if err := db.Find(&dbPackages).Error; err != nil {
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
		lib.Log.Errorf(lib.T_("The field %s cannot be updated."), fieldName)
		return
	}

	updateMap := map[string]interface{}{
		fieldName: value,
	}

	if err := s.db.WithContext(ctx).
		Model(&DBDistroPackage{}).
		Where("container = ? AND name = ?", containerName, name).
		Updates(updateMap).Error; err != nil {
		lib.Log.Error(err)
	}
}

// GetPackageInfoByName возвращает запись (container+name) из таблицы.
func (s *DistroDBService) GetPackageInfoByName(containerName, name string) (PackageInfo, error) {
	var dbp DBDistroPackage
	if err := s.db.
		Where("container = ? AND name = ?", containerName, name).
		First(&dbp).Error; err != nil {
		return PackageInfo{}, err
	}
	return dbp.fromDBModel(), nil
}

// DeletePackagesFromContainer удаляет все записи для указанного контейнера.
func (s *DistroDBService) DeletePackagesFromContainer(ctx context.Context, containerName string) error {
	if err := s.db.WithContext(ctx).
		Where("container = ?", containerName).
		Delete(&DBDistroPackage{}).Error; err != nil {
		return fmt.Errorf(lib.T_("Error deleting container records %s: %v"), containerName, err)
	}
	return nil
}

// applyFilters применяет фильтры
func (s *DistroDBService) applyFilters(db *gorm.DB, filters map[string]interface{}) (*gorm.DB, error) {
	for field, value := range filters {
		if !s.isAllowedField(field, AllowedFilterFields) {
			return nil, fmt.Errorf(lib.T_("Invalid filter field: %s. Available fields: %s."), field, strings.Join(AllowedFilterFields, ", "))
		}
		switch field {
		case "installed", "exporting":
			boolVal, ok := helper.ParseBool(value)
			if ok {
				db = db.Where(fmt.Sprintf("%s = ?", field), boolVal)
			}
		default:
			if strVal, ok := value.(string); ok {
				db = db.Where(fmt.Sprintf("%s LIKE ?", field), "%"+strVal+"%")
			} else {
				db = db.Where(fmt.Sprintf("%s = ?", field), value)
			}
		}
	}
	return db, nil
}

// Проверка, входит ли поле в список разрешённых
func (s *DistroDBService) isAllowedField(field string, allowed []string) bool {
	return slices.Contains(allowed, field)
}

var allowedSortFields = []string{
	"name",
	"version",
	"description",
	"container",
	"installed",
	"exporting",
	"manager",
}

var AllowedFilterFields = []string{
	"name",
	"version",
	"description",
	"container",
	"installed",
	"exporting",
	"manager",
}
