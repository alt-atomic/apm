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

package icon

import (
	"apm/internal/common/app"
	"fmt"
	"log"
	"os"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// DBIcon модель иконки пакета в БД
type DBIcon struct {
	Package   string `gorm:"column:package;primaryKey"`
	Container string `gorm:"column:container;primaryKey;default:''"`
	Icon      []byte `gorm:"column:icon;not null"`
}

// TableName задаёт имя таблицы.
func (DBIcon) TableName() string {
	return "icons"
}

// DBService сервис для работы с иконками в БД
type DBService struct {
	dbManager app.DatabaseManager
	realDb    *gorm.DB
}

var initIconDBMutex sync.Mutex

func (s *DBService) db() (*gorm.DB, error) {
	initIconDBMutex.Lock()
	defer initIconDBMutex.Unlock()

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

		if err = s.realDb.AutoMigrate(&DBIcon{}); err != nil {
			return nil, fmt.Errorf("autoMigrate failed: %w", err)
		}
	}

	return s.realDb, nil
}

// NewIconDBService создаёт новый сервис для работы с иконками в базе данных.
func NewIconDBService(dbManager app.DatabaseManager) *DBService {
	return &DBService{dbManager: dbManager}
}

// SaveIcon сохраняет иконку, игнорируя конфликт если уже существует
func (s *DBService) SaveIcon(pkgName, container string, iconData []byte) error {
	db, err := s.db()
	if err != nil {
		return err
	}

	record := DBIcon{
		Package:   pkgName,
		Container: container,
		Icon:      iconData,
	}
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	return result.Error
}

// GetIcon извлекает сжатую иконку из БД. Если не найдено с контейнером — пробует без контейнера.
func (s *DBService) GetIcon(pkgName, container string) ([]byte, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	var record DBIcon
	err = db.Where("package = ? AND container = ?", pkgName, container).First(&record).Error
	if err != nil && container != "" {
		err = db.Where("package = ? AND container = ''", pkgName).First(&record).Error
	}
	if err != nil {
		return nil, err
	}
	return record.Icon, nil
}

// IconExists проверяет наличие иконки в БД
func (s *DBService) IconExists(pkgName, container string) (bool, error) {
	db, err := s.db()
	if err != nil {
		return false, err
	}

	var count int64
	err = db.Model(&DBIcon{}).Where("package = ? AND container = ?", pkgName, container).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetExistingPackages возвращает множество имён пакетов, уже сохранённых для данного контейнера
func (s *DBService) GetExistingPackages(container string) (map[string]bool, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	var packages []string
	err = db.Model(&DBIcon{}).Where("container = ?", container).Pluck("package", &packages).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(packages))
	for _, p := range packages {
		result[p] = true
	}
	return result, nil
}

// SaveIconsBatch сохраняет иконки игнорируя конфликты
func (s *DBService) SaveIconsBatch(icons []DBIcon) error {
	if len(icons) == 0 {
		return nil
	}
	db, err := s.db()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		batchSize := 500
		for i := 0; i < len(icons); i += batchSize {
			end := i + batchSize
			if end > len(icons) {
				end = len(icons)
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(icons[i:end]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetStats возвращает количество иконок и общий размер данных
func (s *DBService) GetStats() (int, int, error) {
	db, err := s.db()
	if err != nil {
		return 0, 0, err
	}

	var count int64
	var totalSize int64

	if err = db.Model(&DBIcon{}).Count(&count).Error; err != nil {
		return 0, 0, err
	}

	row := db.Model(&DBIcon{}).Select("COALESCE(SUM(LENGTH(icon)), 0)").Row()
	if err = row.Scan(&totalSize); err != nil {
		return 0, 0, err
	}

	return int(count), int(totalSize), nil
}
