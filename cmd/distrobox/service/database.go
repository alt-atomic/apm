package service

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DistroDBService — сервис для операций с базой данных хоста.
type DistroDBService struct {
	dbConn *sql.DB
}

// NewDistroDBService — конструктор сервиса
func NewDistroDBService(db *sql.DB) *DistroDBService {
	return &DistroDBService{
		dbConn: db,
	}
}

// SavePackagesToDB сохраняет список пакетов в таблицу с именем контейнера.
// Таблица создаётся, если не существует, затем очищается, и в неё вставляются новые записи пакетами по 1000.
func (s *DistroDBService) SavePackagesToDB(ctx context.Context, containerName string, packages []PackageInfo) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)
	tableName := fmt.Sprintf("\"%s\"", containerName)

	// Создаем таблицу, если её нет.
	createQuery := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		packageName TEXT,
		version TEXT,
		description TEXT,
		installed INTEGER,
		exporting INTEGER,
		manager TEXT
	)`, tableName)
	if _, err := s.dbConn.Exec(createQuery); err != nil {
		return err
	}

	// Очищаем таблицу.
	deleteQuery := fmt.Sprintf("DELETE FROM %s", tableName)
	if _, err := s.dbConn.Exec(deleteQuery); err != nil {
		return err
	}

	// Начинаем транзакцию.
	tx, err := s.dbConn.Begin()
	if err != nil {
		return err
	}

	batchSize := 1000
	n := len(packages)
	for i := 0; i < n; i += batchSize {
		end := i + batchSize
		if end > n {
			end = n
		}
		batch := packages[i:end]

		var placeholders []string
		var args []interface{}
		for _, pkg := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
			var installed, exporting int
			if pkg.Installed {
				installed = 1
			}
			if pkg.Exporting {
				exporting = 1
			}
			args = append(args, pkg.PackageName, pkg.Version, pkg.Description, installed, exporting, pkg.Manager)
		}
		query := fmt.Sprintf("INSERT INTO %s (packageName, version, description, installed, exporting, manager) VALUES %s",
			tableName, strings.Join(placeholders, ","))
		if _, err = tx.Exec(query, args...); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// ContainerDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func (s *DistroDBService) ContainerDatabaseExist(ctx context.Context, containerName string) error {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	var count int
	err := s.dbConn.QueryRow(query).Scan(&count)
	if err != nil {
		return err
	}

	return nil
}

// CountTotalPackages выполняет запрос COUNT(*) для таблицы с пакетами, применяя фильтры.
func (s *DistroDBService) CountTotalPackages(containerName string, filters map[string]interface{}) (int, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	var args []interface{}
	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			// Проверяем: если поле — installed или exporting, пытаемся его трактовать как bool.
			if field == "installed" || field == "exporting" {
				boolVal, ok := helper.ParseBool(value)
				if !ok {
					continue
				}
				conditions = append(conditions, fmt.Sprintf("%s = ?", field))
				if boolVal {
					args = append(args, 1)
				} else {
					args = append(args, 0)
				}
			} else {
				// Остальные поля
				if strVal, ok := value.(string); ok {
					// Для строк, как и раньше, используем LIKE
					conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
					args = append(args, "%"+strVal+"%")
				} else {
					// Для всего прочего — точное совпадение
					conditions = append(conditions, fmt.Sprintf("%s = ?", field))
					args = append(args, value)
				}
			}
		}
		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
	}
	var total int
	err := s.dbConn.QueryRow(query, args...).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// QueryPackages возвращает пакеты из таблицы контейнера с возможностью фильтрации, сортировки, limit и offset.
func (s *DistroDBService) QueryPackages(containerName string, filters map[string]interface{}, sortField, sortOrder string, limit, offset int64) ([]PackageInfo, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)

	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s", tableName)
	var args []interface{}

	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			// Проверяем: если поле — installed или exporting, пытаемся его трактовать как bool.
			if field == "installed" || field == "exporting" {
				boolVal, ok := helper.ParseBool(value)
				if !ok {
					continue
				}
				conditions = append(conditions, fmt.Sprintf("%s = ?", field))
				if boolVal {
					args = append(args, 1)
				} else {
					args = append(args, 0)
				}
			} else {
				// Остальные поля
				if strVal, ok := value.(string); ok {
					// Для строк, как и раньше, используем LIKE
					conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
					args = append(args, "%"+strVal+"%")
				} else {
					// Для всего прочего — точное совпадение
					conditions = append(conditions, fmt.Sprintf("%s = ?", field))
					args = append(args, value)
				}
			}
		}
		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	// Сортировка
	if sortField != "" {
		upperOrder := strings.ToUpper(sortOrder)
		if upperOrder != "ASC" && upperOrder != "DESC" {
			upperOrder = "ASC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", sortField, upperOrder)
	}

	// Лимит
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	rows, err := s.dbConn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			lib.Log.Error(err)
		}
	}(rows)

	var packages []PackageInfo
	for rows.Next() {
		var pkg PackageInfo
		var installed, exporting int
		if err := rows.Scan(&pkg.PackageName, &pkg.Version, &pkg.Description, &installed, &exporting, &pkg.Manager); err != nil {
			return nil, err
		}
		pkg.Installed = installed != 0
		pkg.Exporting = exporting != 0
		packages = append(packages, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

// FindPackagesByName ищет пакеты в таблице контейнера по неточному совпадению имени.
// Используется оператор LIKE для поиска, возвращается срез найденных записей.
func (s *DistroDBService) FindPackagesByName(containerName string, partialName string) ([]PackageInfo, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	// Формируем запрос с условием LIKE
	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s WHERE packageName LIKE ?", tableName)
	searchPattern := "%" + partialName + "%"

	rows, err := s.dbConn.Query(query, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []PackageInfo
	for rows.Next() {
		var pkg PackageInfo
		var installed, exporting int
		if err := rows.Scan(&pkg.PackageName, &pkg.Version, &pkg.Description, &installed, &exporting, &pkg.Manager); err != nil {
			return nil, err
		}
		pkg.Installed = installed != 0
		pkg.Exporting = exporting != 0
		packages = append(packages, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

// UpdatePackageField обновляет значение одного поля (installed или exporting) для пакета с указанным packageName в таблице контейнера.
func (s *DistroDBService) UpdatePackageField(ctx context.Context, containerName, packageName, fieldName string, value bool) {
	// Разрешенные поля для обновления.
	allowedFields := map[string]bool{
		"installed": true,
		"exporting": true,
	}
	if !allowedFields[fieldName] {
		lib.Log.Errorf("поле %s нельзя обновлять", fieldName)
	}

	// Экранируем имя таблицы.
	tableName := fmt.Sprintf("\"%s\"", containerName)
	// Формируем запрос. Например: UPDATE "containerName" SET installed = ? WHERE packageName = ?
	updateQuery := fmt.Sprintf("UPDATE %s SET %s = ? WHERE packageName = ?", tableName, fieldName)

	var intVal int
	if value {
		intVal = 1
	} else {
		intVal = 0
	}

	_, err := s.dbConn.Exec(updateQuery, intVal, packageName)
	if err != nil {
		lib.Log.Error(err.Error())
	}
}

// GetPackageInfoByName возвращает запись пакета с указанным packageName из таблицы контейнера.
func (s *DistroDBService) GetPackageInfoByName(containerName, packageName string) (PackageInfo, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s WHERE packageName = ?", tableName)

	var pkg PackageInfo
	var installed, exporting int
	err := s.dbConn.QueryRow(query, packageName).Scan(&pkg.PackageName, &pkg.Version, &pkg.Description, &installed, &exporting, &pkg.Manager)
	if err != nil {
		return PackageInfo{}, err
	}

	pkg.Installed = installed != 0
	pkg.Exporting = exporting != 0

	return pkg, nil
}

// DeleteContainerTable удаляет таблицу для указанного контейнера.
func (s *DistroDBService) DeleteContainerTable(ctx context.Context, containerName string) error {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	if _, err := s.dbConn.Exec(query); err != nil {
		return fmt.Errorf("ошибка удаления таблицы %s: %v", containerName, err)
	}

	return nil
}
