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
	dbConn            *sql.DB
	packagesTableName string
}

// NewDistroDBService — конструктор сервиса
func NewDistroDBService(db *sql.DB) *DistroDBService {
	return &DistroDBService{
		packagesTableName: "distrobox_packages",
		dbConn:            db,
	}
}

// SavePackagesToDB сохраняет список пакетов в таблицу с именем контейнера.
// Таблица создаётся, если не существует, затем очищается, и в неё вставляются новые записи пакетами по 1000.
func (s *DistroDBService) SavePackagesToDB(ctx context.Context, containerName string, packages []PackageInfo) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.SavePackagesToDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.SavePackagesToDB"))

	if len(containerName) == 0 {
		return fmt.Errorf("поле container не может быть пустым при сохранении пакетов в базу данных")
	}

	// Создаем таблицу, если её нет. Таблица содержит поле container.
	createQuery := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		container TEXT,
		name TEXT,
		version TEXT,
		description TEXT,
		installed INTEGER,
		exporting INTEGER,
		manager TEXT
	)`, s.packagesTableName)
	if _, err := s.dbConn.Exec(createQuery); err != nil {
		return err
	}

	// Очищаем записи для конкретного контейнера, не затрагивая данные других контейнеров.
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE container = ?", s.packagesTableName)
	if _, err := s.dbConn.Exec(deleteQuery, containerName); err != nil {
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
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?)")
			var installed, exporting int
			if pkg.Installed {
				installed = 1
			}
			if pkg.Exporting {
				exporting = 1
			}
			// Первый параметр – имя контейнера.
			args = append(args, containerName, pkg.Name, pkg.Version, pkg.Description, installed, exporting, pkg.Manager)
		}
		query := fmt.Sprintf("INSERT INTO %s (container, name, version, description, installed, exporting, manager) VALUES %s",
			s.packagesTableName, strings.Join(placeholders, ","))
		if _, err = tx.Exec(query, args...); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// DatabaseExist проверяет, существует ли база данных и содержит ли она хотя бы одну запись.
func (s *DistroDBService) DatabaseExist(ctx context.Context) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.packagesTableName)
	var count int
	err := s.dbConn.QueryRow(query).Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("в базе данных отсутствуют записи, необходимо создать или обновить любой контейнер")
		}
		return err
	}

	if count == 0 {
		return fmt.Errorf("в базе данных отсутствуют записи, необходимо создать или обновить любой контейнер")
	}

	return nil
}

// ContainerDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func (s *DistroDBService) ContainerDatabaseExist(ctx context.Context, containerName string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE container = ?", s.packagesTableName)
	var count int
	err := s.dbConn.QueryRow(query, containerName).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("нет записей для контейнера %s", containerName)
	}
	return nil
}

// CountTotalPackages выполняет запрос COUNT(*) для таблицы с пакетами, применяя фильтры.
func (s *DistroDBService) CountTotalPackages(containerName string, filters map[string]interface{}) (int, error) {
	// Начинаем базовый запрос без условия.
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.packagesTableName)
	var conditions []string
	var args []interface{}

	// Если containerName задан, добавляем условие фильтрации.
	if containerName != "" {
		conditions = append(conditions, "container = ?")
		args = append(args, containerName)
	}

	// Формируем дополнительные условия по фильтрам.
	for field, value := range filters {
		// Если поле installed или exporting – пытаемся трактовать как bool.
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
			// Для остальных полей: если строка – используем LIKE, иначе точное совпадение.
			if strVal, ok := value.(string); ok {
				conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
				args = append(args, "%"+strVal+"%")
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = ?", field))
				args = append(args, value)
			}
		}
	}
	// Если условия сформированы, добавляем их к запросу с конструкцией WHERE.
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
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
	// Начинаем базовый запрос без условия WHERE.
	query := fmt.Sprintf("SELECT name, version, description, container, installed, exporting, manager FROM %s", s.packagesTableName)
	var conditions []string
	var args []interface{}

	// Если containerName задан, добавляем условие фильтрации по нему.
	if containerName != "" {
		conditions = append(conditions, "container = ?")
		args = append(args, containerName)
	}

	// Формируем условия по дополнительным фильтрам.
	for field, value := range filters {
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
			if strVal, ok := value.(string); ok {
				conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
				args = append(args, "%"+strVal+"%")
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = ?", field))
				args = append(args, value)
			}
		}
	}

	// Если условия сформированы, добавляем их к запросу.
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Добавляем сортировку, если задана.
	if sortField != "" {
		upperOrder := strings.ToUpper(sortOrder)
		if upperOrder != "ASC" && upperOrder != "DESC" {
			upperOrder = "ASC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", sortField, upperOrder)
	}

	// Добавляем LIMIT и OFFSET, если заданы.
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
		if err := rows.Scan(&pkg.Name, &pkg.Version, &pkg.Description, &pkg.Container, &installed, &exporting, &pkg.Manager); err != nil {
			return nil, err
		}
		pkg.Installed = installed != 0
		pkg.Exporting = exporting != 0
		packages = append(packages, pkg)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

// FindPackagesByName ищет пакеты в таблице контейнера по неточному совпадению имени.
// Используется оператор LIKE для поиска, возвращается срез найденных записей.
func (s *DistroDBService) FindPackagesByName(containerName string, partialName string) ([]PackageInfo, error) {
	query := fmt.Sprintf("SELECT name, version, description, container, installed, exporting, manager FROM %s", s.packagesTableName)
	var conditions []string
	var args []interface{}

	if containerName != "" {
		conditions = append(conditions, "container = ?")
		args = append(args, containerName)
	}

	if partialName != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+partialName+"%")
	}

	// Если есть условия, формируем часть WHERE
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := s.dbConn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []PackageInfo
	for rows.Next() {
		var pkg PackageInfo
		var installed, exporting int
		if err = rows.Scan(&pkg.Name, &pkg.Version, &pkg.Description, &pkg.Container, &installed, &exporting, &pkg.Manager); err != nil {
			return nil, err
		}
		pkg.Installed = installed != 0
		pkg.Exporting = exporting != 0
		packages = append(packages, pkg)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

// UpdatePackageField обновляет значение одного поля (installed или exporting) для пакета с указанным name в таблице контейнера.
func (s *DistroDBService) UpdatePackageField(ctx context.Context, containerName, name, fieldName string, value bool) {
	// Разрешенные поля для обновления.
	allowedFields := map[string]bool{
		"installed": true,
		"exporting": true,
	}
	if !allowedFields[fieldName] {
		lib.Log.Errorf("поле %s нельзя обновлять", fieldName)
		return
	}

	// Формируем запрос с фильтрацией по container и name.
	updateQuery := fmt.Sprintf("UPDATE %s SET %s = ? WHERE container = ? AND name = ?", s.packagesTableName, fieldName)

	var intVal int
	if value {
		intVal = 1
	} else {
		intVal = 0
	}

	_, err := s.dbConn.Exec(updateQuery, intVal, containerName, name)
	if err != nil {
		lib.Log.Error(err.Error())
	}
}

// GetPackageInfoByName возвращает запись пакета с указанным name из таблицы, фильтруя по container.
func (s *DistroDBService) GetPackageInfoByName(containerName, name string) (PackageInfo, error) {
	query := fmt.Sprintf("SELECT name, version, description, container, installed, exporting, manager FROM %s WHERE container = ? AND name = ?", s.packagesTableName)

	var pkg PackageInfo
	var installed, exporting int
	err := s.dbConn.QueryRow(query, containerName, name).Scan(&pkg.Name, &pkg.Version, &pkg.Description, &pkg.Container, &installed, &exporting, &pkg.Manager)
	if err != nil {
		return PackageInfo{}, err
	}

	pkg.Installed = installed != 0
	pkg.Exporting = exporting != 0

	return pkg, nil
}

// DeletePackagesFromContainer удаляет таблицу для указанного контейнера.
func (s *DistroDBService) DeletePackagesFromContainer(ctx context.Context, containerName string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE container = ?", s.packagesTableName)
	if _, err := s.dbConn.Exec(query, containerName); err != nil {
		return fmt.Errorf("ошибка удаления записей контейнера %s: %v", containerName, err)
	}

	return nil
}
