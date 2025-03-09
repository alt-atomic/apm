package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"fmt"
	"time"
)

// ImageHistory описывает сведения об образе.
type ImageHistory struct {
	ImageName string `json:"imageName"`
	Config    string `json:"config"`
	ImageDate string `json:"imageDate"`
}

// SaveImageToDB сохраняет историю образов в БД.
func SaveImageToDB(ctx context.Context, imageHistory ImageHistory) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	tableName := "host_image_history"

	createQuery := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		imagename TEXT,
		config TEXT,
		imagedate TIMESTAMP
	)`, tableName)

	if _, err := lib.DB.Exec(createQuery); err != nil {
		return fmt.Errorf("ошибка создания таблицы: %v", err)
	}

	tx, err := lib.DB.Begin()
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

	if _, err := stmt.Exec(imageHistory.ImageName, imageHistory.Config, parsedDate); err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка вставки данных: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при коммите транзакции: %v", err)
	}

	return nil
}

// GetImageHistoriesFiltered возвращает все записи из таблицы host_image_history,
// сортируя их по дате (новые записи первыми) и фильтруя по названию образа.
func GetImageHistoriesFiltered(ctx context.Context, imageNameFilter string) ([]ImageHistory, error) {
	tableName := "host_image_history"

	query := fmt.Sprintf("SELECT imagename, config, imagedate FROM %s", tableName)
	var args []interface{}

	if imageNameFilter != "" {
		query += " WHERE imagename LIKE ?"
		args = append(args, "%"+imageNameFilter+"%")
	}

	query += " ORDER BY imagedate DESC"

	rows, err := lib.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer rows.Close()

	var histories []ImageHistory

	for rows.Next() {
		var imageName, config string
		var imageDate time.Time

		if err := rows.Scan(&imageName, &config, &imageDate); err != nil {
			return nil, fmt.Errorf("ошибка чтения данных: %v", err)
		}

		history := ImageHistory{
			ImageName: imageName,
			Config:    config,
			ImageDate: imageDate.Format(time.RFC3339),
		}
		histories = append(histories, history)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %v", err)
	}

	return histories, nil
}
