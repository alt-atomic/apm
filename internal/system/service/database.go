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
	"apm/internal/common/app"
	"apm/internal/common/build"
	"apm/internal/common/reply"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ImageHistory struct {
	ImageName string        `json:"image"`
	Config    *build.Config `json:"config"`
	ImageDate string        `json:"date"`
}

type DBHistory struct {
	ImageName  string    `gorm:"column:imagename;primaryKey"`
	ImageDate  time.Time `gorm:"column:imagedate;primaryKey"`
	ConfigJSON string    `gorm:"column:config"`
}

type HostDBService struct {
	db *gorm.DB
}

// NewHostDBService — конструктор сервиса
func NewHostDBService(db *sql.DB) (*HostDBService, error) {
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
		return nil, fmt.Errorf("ошибка подключения к SQLite через GORM: %w", err)
	}

	// автоматическая миграция
	if err = gormDB.AutoMigrate(&DBHistory{}); err != nil {
		return nil, fmt.Errorf("ошибка миграции структуры таблицы: %w", err)
	}

	return &HostDBService{
		db: gormDB,
	}, nil
}

// TableName - задаём нужное имя таблицы
func (DBHistory) TableName() string {
	return "host_image_history"
}

// fromDBModel — превращает DBHistory (структура БД) в бизнес-структуру ImageHistory
func (dbh DBHistory) fromDBModel() (ImageHistory, error) {
	var err error
	var cfg build.Config
	if cfg, err = build.ParseJsonData([]byte(dbh.ConfigJSON)); err != nil {
		return ImageHistory{}, fmt.Errorf(app.T_("Config conversion error: %v"), err)
	}

	return ImageHistory{
		ImageName: dbh.ImageName,
		Config:    &cfg,
		ImageDate: dbh.ImageDate.Format(time.RFC3339),
	}, nil
}

// toDBModel — превращает бизнес-структуру ImageHistory в DBHistory (для сохранения в БД)
func (ih ImageHistory) toDBModel() (DBHistory, error) {
	parsedDate, err := time.Parse(time.RFC3339, ih.ImageDate)
	if err != nil {
		return DBHistory{}, fmt.Errorf(app.T_("Error parsing date %s: %v"), ih.ImageDate, err)
	}

	cfgBytes, err := json.Marshal(ih.Config)
	if err != nil {
		return DBHistory{}, fmt.Errorf(app.T_("Error serializing config: %v"), err)
	}

	return DBHistory{
		ImageName:  ih.ImageName,
		ConfigJSON: string(cfgBytes),
		ImageDate:  parsedDate,
	}, nil
}

// SaveImageToDB сохраняет историю образов в БД (через GORM).
func (h *HostDBService) SaveImageToDB(ctx context.Context, imageHistory ImageHistory) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.SaveImageToDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.SaveImageToDB"))

	// Преобразуем в модель БД
	dbHist, err := imageHistory.toDBModel()
	if err != nil {
		return err
	}

	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if errCreate := tx.Create(&dbHist).Error; errCreate != nil {
			return fmt.Errorf(app.T_("Error inserting data: %v"), errCreate)
		}
		return nil
	})

	return err
}

// GetImageHistoriesFiltered возвращает все записи по имени
func (h *HostDBService) GetImageHistoriesFiltered(ctx context.Context, imageNameFilter string, limit, offset int) ([]ImageHistory, error) {
	query := h.db.WithContext(ctx).Model(&DBHistory{})

	if imageNameFilter != "" {
		query = query.Where("imagename LIKE ?", "%"+imageNameFilter+"%")
	}

	query = query.Order("imagedate DESC").
		Limit(limit).
		Offset(offset)

	var dbHistories []DBHistory
	if err := query.Find(&dbHistories).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, errors.New(app.T_("History not found"))
		}
		return nil, fmt.Errorf(app.T_("Query execution error: %v"), err)
	}

	var histories []ImageHistory
	for _, dbh := range dbHistories {
		ih, err := dbh.fromDBModel()
		if err != nil {
			return nil, err
		}
		histories = append(histories, ih)
	}

	return histories, nil
}

// CountImageHistoriesFiltered — возвращает количество записей с фильтром по имени образа.
func (h *HostDBService) CountImageHistoriesFiltered(ctx context.Context, imageNameFilter string) (int, error) {
	query := h.db.WithContext(ctx).Model(&DBHistory{})

	if imageNameFilter != "" {
		query = query.Where("imagename LIKE ?", "%"+imageNameFilter+"%")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return 0, errors.New(app.T_("History not found"))
		}
		return 0, fmt.Errorf(app.T_("Query execution error: %v"), err)
	}

	return int(count), nil
}

// IsLatestConfigSame сравнивает newConfig с последним сохранённым в БД.
func (h *HostDBService) IsLatestConfigSame(ctx context.Context, newConfig build.Config) (bool, error) {
	var dbHist DBHistory
	err := h.db.WithContext(ctx).Model(&DBHistory{}).
		Order("imagedate DESC").
		Limit(1).
		Take(&dbHist).Error
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		if strings.Contains(err.Error(), "record not found") {
			return false, nil
		}
		return false, fmt.Errorf(app.T_("Query execution error: %v"), err)
	}

	var latestConfig build.Config
	if latestConfig, err = build.ParseJsonData([]byte(dbHist.ConfigJSON)); err != nil {
		return false, fmt.Errorf(app.T_("History config conversion error: %v"), err)
	}

	return reflect.DeepEqual(newConfig, latestConfig), nil
}
