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
	"apm/cmd/common/reply"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// HostDBService — сервис для операций с базой данных хоста.
type HostDBService struct {
	historyTableName string
	dbConn           *sql.DB
}

// NewHostDBService — конструктор сервиса
func NewHostDBService(db *sql.DB) *HostDBService {
	return &HostDBService{
		historyTableName: "host_image_history",
		dbConn:           db,
	}
}

// ImageHistory описывает сведения об образе.
// Здесь поле Config хранится в виде ссылки на структуру Config.
type ImageHistory struct {
	ImageName string  `json:"image"`
	Config    *Config `json:"config"`
	ImageDate string  `json:"date"`
}

// SaveImageToDB сохраняет историю образов в БД.
// Перед сохранением объект Config сериализуется в JSON-строку.
func (h *HostDBService) SaveImageToDB(ctx context.Context, imageHistory ImageHistory) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.SaveImageToDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.SaveImageToDB"))

	tableName := "host_image_history"

	createQuery := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		imagename TEXT,
		config TEXT,
		imagedate TIMESTAMP
	)`, h.historyTableName)

	if _, err := h.dbConn.Exec(createQuery); err != nil {
		return fmt.Errorf("ошибка создания таблицы: %v", err)
	}

	// Сериализуем конфиг в JSON-строку.
	configJSON, err := json.Marshal(imageHistory.Config)
	if err != nil {
		return fmt.Errorf("ошибка сериализации конфига: %v", err)
	}

	tx, err := h.dbConn.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %v", err)
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT INTO %s (imagename, config, imagedate) VALUES (?, ?, ?)`, tableName))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка подготовки запроса: %v", err)
	}
	defer stmt.Close()

	parsedDate, err := time.Parse(time.RFC3339, imageHistory.ImageDate)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка парсинга даты %s: %v", imageHistory.ImageDate, err)
	}

	if _, err = stmt.Exec(imageHistory.ImageName, string(configJSON), parsedDate); err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка вставки данных: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при коммите транзакции: %v", err)
	}

	return nil
}

// GetImageHistoriesFiltered возвращает все записи из таблицы host_image_history,
// сортируя их по дате (новые записи первыми), фильтруя по названию образа,
// а также применяя limit и offset для пагинации.
func (h *HostDBService) GetImageHistoriesFiltered(ctx context.Context, imageNameFilter string, limit int64, offset int64) ([]ImageHistory, error) {
	query := fmt.Sprintf("SELECT imagename, config, imagedate FROM %s", h.historyTableName)
	var args []interface{}

	if imageNameFilter != "" {
		query += " WHERE imagename LIKE ?"
		args = append(args, "%"+imageNameFilter+"%")
	}

	query += " ORDER BY imagedate DESC"
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.dbConn.QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "doesn't exist") {
			return nil, fmt.Errorf("история не найдена")
		}
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer rows.Close()

	var histories []ImageHistory

	for rows.Next() {
		var imageName string
		var configJSON string
		var imageDate time.Time

		if err = rows.Scan(&imageName, &configJSON, &imageDate); err != nil {
			return nil, fmt.Errorf("ошибка чтения данных: %v", err)
		}

		var cfg Config
		if err = json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return nil, fmt.Errorf("ошибка преобразования конфига: %v", err)
		}

		history := ImageHistory{
			ImageName: imageName,
			Config:    &cfg,
			ImageDate: imageDate.Format(time.RFC3339),
		}
		histories = append(histories, history)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %v", err)
	}

	return histories, nil
}

// CountImageHistoriesFiltered возвращает количество записей
// фильтруя по названию образа.
func (h *HostDBService) CountImageHistoriesFiltered(ctx context.Context, imageNameFilter string) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", h.historyTableName)
	var args []interface{}

	if imageNameFilter != "" {
		query += " WHERE imagename LIKE ?"
		args = append(args, "%"+imageNameFilter+"%")
	}

	var count int
	err := h.dbConn.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "doesn't exist") {
			return 0, fmt.Errorf("история не найдена")
		}
		return 0, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}

	return count, nil
}

// IsLatestConfigSame принимает конфигурацию newConfig, получает из БД самый последний сохранённый конфиг
// и сравнивает их. Если они совпадают, возвращает true, иначе false.
func (h *HostDBService) IsLatestConfigSame(ctx context.Context, newConfig Config) (bool, error) {
	query := fmt.Sprintf("SELECT config FROM %s ORDER BY imagedate DESC LIMIT 1", h.historyTableName)

	var configJSON string
	err := h.dbConn.QueryRowContext(ctx, query).Scan(&configJSON)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "doesn't exist") {
			return false, nil
		}
		return false, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}

	var latestConfig Config
	if err = json.Unmarshal([]byte(configJSON), &latestConfig); err != nil {
		return false, fmt.Errorf("ошибка преобразования конфига из истории: %v", err)
	}

	if reflect.DeepEqual(newConfig, latestConfig) {
		return true, nil
	}

	return false, nil
}
