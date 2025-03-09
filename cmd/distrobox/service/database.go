package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"fmt"
	"strings"
)

// SavePackagesToDB сохраняет список пакетов в таблицу с именем контейнера.
// Таблица создаётся, если не существует, затем очищается, и в неё вставляются новые записи пакетами по 1000.
func SavePackagesToDB(ctx context.Context, containerName string, packages []PackageInfo) error {
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
	if _, err := lib.DB.Exec(createQuery); err != nil {
		return err
	}

	// Очищаем таблицу.
	deleteQuery := fmt.Sprintf("DELETE FROM %s", tableName)
	if _, err := lib.DB.Exec(deleteQuery); err != nil {
		return err
	}

	// Начинаем транзакцию.
	tx, err := lib.DB.Begin()
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
		if _, err := tx.Exec(query, args...); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// ContainerDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func ContainerDatabaseExist(ctx context.Context, containerName string) error {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	var count int
	err := lib.DB.QueryRow(query).Scan(&count)
	if err != nil {
		return err
	}

	return nil
}

// CountTotalPackages выполняет запрос COUNT(*) для таблицы с пакетами, применяя фильтры.
func CountTotalPackages(containerName string, filters map[string]interface{}) (int, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	var args []interface{}
	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			conditions = append(conditions, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		}
		whereClause := strings.Join(conditions, " AND ")
		query += " WHERE " + whereClause
	}
	var total int
	err := lib.DB.QueryRow(query, args...).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// QueryPackages возвращает пакеты из таблицы контейнера с возможностью фильтрации, сортировки, limit и offset.
// filters - словарь, где ключ — имя поля, значение - искомое значение (будет использовано условие "=").
// sortField - имя поля для сортировки (если пустое, сортировка не применяется).
// sortOrder - "ASC" или "DESC" (по умолчанию ASC, если задано неверно, то ASC).
// limit и offset - ограничения выборки. Если limit <= 0, то ограничение не применяется.
func QueryPackages(containerName string, filters map[string]interface{}, sortField, sortOrder string, limit int64, offset int64) ([]PackageInfo, error) {
	// Экранируем имя таблицы
	tableName := fmt.Sprintf("\"%s\"", containerName)

	// Базовый запрос
	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s", tableName)
	var args []interface{}

	// Формируем WHERE-условие, если есть фильтры.
	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			// Предполагаем, что имя поля корректное (вы можете добавить проверку допустимых полей).
			conditions = append(conditions, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		}
		whereClause := strings.Join(conditions, " AND ")
		query += " WHERE " + whereClause
	}

	// Добавляем сортировку, если задано поле сортировки.
	if sortField != "" {
		// Проверяем направление сортировки.
		upperOrder := strings.ToUpper(sortOrder)
		if upperOrder != "ASC" && upperOrder != "DESC" {
			upperOrder = "ASC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", sortField, upperOrder)
	}

	// Добавляем LIMIT и OFFSET, если limit > 0.
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	// Выполняем запрос.
	rows, err := lib.DB.Query(query, args...)
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

// FindPackagesByName ищет пакеты в таблице контейнера по неточному совпадению имени.
// Используется оператор LIKE для поиска, возвращается срез найденных записей.
func FindPackagesByName(containerName string, partialName string) ([]PackageInfo, error) {
	// Экранируем имя таблицы
	tableName := fmt.Sprintf("\"%s\"", containerName)
	// Формируем запрос с условием LIKE
	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s WHERE packageName LIKE ?", tableName)
	// Используем шаблон для неточного совпадения
	searchPattern := "%" + partialName + "%"

	rows, err := lib.DB.Query(query, searchPattern)
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
func UpdatePackageField(ctx context.Context, containerName, packageName, fieldName string, value bool) {
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

	_, err := lib.DB.Exec(updateQuery, intVal, packageName)
	if err != nil {
		lib.Log.Error(err.Error())
	}
}

// GetPackageInfoByName возвращает запись пакета с указанным packageName из таблицы контейнера.
func GetPackageInfoByName(containerName, packageName string) (PackageInfo, error) {
	tableName := fmt.Sprintf("\"%s\"", containerName)
	query := fmt.Sprintf("SELECT packageName, version, description, installed, exporting, manager FROM %s WHERE packageName = ?", tableName)

	var pkg PackageInfo
	var installed, exporting int
	err := lib.DB.QueryRow(query, packageName).Scan(&pkg.PackageName, &pkg.Version, &pkg.Description, &installed, &exporting, &pkg.Manager)
	if err != nil {
		return PackageInfo{}, err
	}

	pkg.Installed = installed != 0
	pkg.Exporting = exporting != 0

	return pkg, nil
}
