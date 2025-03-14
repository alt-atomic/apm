package apt

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// PackageDBService — сервис для операций с базой данных пакетов.
type PackageDBService struct {
	tableName string
	dbConn    *sql.DB
}

// NewPackageDBService — конструктор сервиса, где задаётся имя таблицы.
func NewPackageDBService(db *sql.DB) *PackageDBService {
	return &PackageDBService{
		tableName: "host_image_packages",
		dbConn:    db,
	}
}

// syncDBMutex защищает операции синхронизации базы пакетов.
var syncDBMutex sync.Mutex

// SavePackagesToDB сохраняет список пакетов
func (s *PackageDBService) SavePackagesToDB(ctx context.Context, packages []Package) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	// Создаем таблицу, если её нет.
	createQuery := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		name TEXT,
		section TEXT,
		installed_size INTEGER,
		maintainer TEXT,
		version TEXT,
		versionInstalled TEXT,
		depends TEXT,
		provides TEXT,
		size INTEGER,
		filename TEXT,
		description TEXT,
		changelog TEXT,
		installed INTEGER
	)`, s.tableName)
	if _, err := s.dbConn.Exec(createQuery); err != nil {
		return fmt.Errorf("ошибка создания таблицы: %w", err)
	}

	// Очищаем таблицу.
	deleteQuery := fmt.Sprintf("DELETE FROM %s", s.tableName)
	if _, err := s.dbConn.Exec(deleteQuery); err != nil {
		return fmt.Errorf("ошибка очистки таблицы: %w", err)
	}

	// Начинаем транзакцию.
	tx, err := s.dbConn.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
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
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			dependsStr := strings.Join(pkg.Depends, ",")
			providersStr := strings.Join(pkg.Provides, ",")
			var installed int
			if pkg.Installed {
				installed = 1
			} else {
				installed = 0
			}
			args = append(args,
				pkg.Name,
				pkg.Section,
				pkg.InstalledSize,
				pkg.Maintainer,
				pkg.Version,
				pkg.VersionInstalled,
				dependsStr,
				providersStr,
				pkg.Size,
				pkg.Filename,
				pkg.Description,
				pkg.Changelog,
				installed,
			)
		}

		query := fmt.Sprintf("INSERT INTO %s (name, section, installed_size, maintainer, version, versionInstalled, depends, provides, size, filename, description, changelog, installed) VALUES %s",
			s.tableName, strings.Join(placeholders, ","))
		if _, err = tx.Exec(query, args...); err != nil {
			errRollback := tx.Rollback()
			if errRollback != nil {
				return errRollback
			}
			return fmt.Errorf("ошибка вставки пакетов: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

// GetPackageByName возвращает запись пакета
func (s *PackageDBService) GetPackageByName(ctx context.Context, packageName string) (Package, error) {
	query := fmt.Sprintf(`
		SELECT name, section, installed_size, maintainer, version, versionInstalled, depends, provides, size, filename, description, changelog, installed 
		FROM %s 
		WHERE name = ?`, s.tableName)

	var pkg Package
	var dependsStr string
	var providersStr string
	var installed int

	err := s.dbConn.QueryRowContext(ctx, query, packageName).Scan(
		&pkg.Name,
		&pkg.Section,
		&pkg.InstalledSize,
		&pkg.Maintainer,
		&pkg.Version,
		&pkg.VersionInstalled,
		&dependsStr,
		&providersStr,
		&pkg.Size,
		&pkg.Filename,
		&pkg.Description,
		&pkg.Changelog,
		&installed,
	)
	if err != nil {
		return Package{}, fmt.Errorf("не удалось получить информацию о пакете %s", packageName)
	}

	// Преобразуем строку зависимостей в срез.
	if dependsStr != "" {
		pkg.Depends = strings.Split(dependsStr, ",")
	} else {
		pkg.Depends = []string{}
	}

	if providersStr != "" {
		pkg.Provides = strings.Split(providersStr, ",")
	} else {
		pkg.Provides = []string{}
	}

	pkg.Installed = installed != 0

	return pkg, nil
}

// SyncPackageInstallationInfo синхронизирует базу пакетов с результатом выполнения apt.GetInstalledPackages().
func (s *PackageDBService) SyncPackageInstallationInfo(ctx context.Context, installedPackages map[string]string) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()

	tx, err := s.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	createTempTableQuery := `
        CREATE TEMPORARY TABLE tmp_installed (
            name TEXT PRIMARY KEY,
            version TEXT
        );
    `
	if _, err = tx.ExecContext(ctx, createTempTableQuery); err != nil {
		return fmt.Errorf("ошибка создания временной таблицы: %w", err)
	}

	var placeholders []string
	var args []interface{}
	for name, version := range installedPackages {
		placeholders = append(placeholders, "(?, ?)")
		args = append(args, name, version)
	}

	if len(placeholders) > 0 {
		insertQuery := fmt.Sprintf("INSERT INTO tmp_installed (name, version) VALUES %s", strings.Join(placeholders, ", "))
		if _, err = tx.ExecContext(ctx, insertQuery, args...); err != nil {
			return fmt.Errorf("ошибка пакетной вставки во временную таблицу: %w", err)
		}
	}

	updateQuery := fmt.Sprintf(`
        UPDATE %s
        SET 
            installed = CASE 
                WHEN EXISTS (SELECT 1 FROM tmp_installed t WHERE t.name = %s.name) THEN 1 
                ELSE 0 
            END,
            versionInstalled = COALESCE(
                (SELECT t.version FROM tmp_installed t WHERE t.name = %s.name), 
                ''
            )
    `, s.tableName, s.tableName, s.tableName)
	if _, err = tx.ExecContext(ctx, updateQuery); err != nil {
		return fmt.Errorf("ошибка обновления пакетов: %w", err)
	}

	// 4. Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

// SearchPackagesByName ищет пакеты в таблице по части названия.
// Параметр `installed` определяет, нужно ли показывать только установленные пакеты.
func (s *PackageDBService) SearchPackagesByName(ctx context.Context, namePart string, installed bool) ([]Package, error) {
	baseQuery := fmt.Sprintf(`
		SELECT 
			name, 
			section, 
			installed_size, 
			maintainer, 
			version, 
			versionInstalled, 
			depends,
		    provides,
			size, 
			filename, 
			description, 
			changelog, 
			installed
		FROM %s
		WHERE name LIKE ?
	`, s.tableName)

	// Если нужно искать только среди установленных
	if installed {
		baseQuery += " AND installed = 1"
	}

	// Подготавливаем шаблон для поиска, например "%имя%"
	searchPattern := "%" + namePart + "%"

	rows, err := s.dbConn.QueryContext(ctx, baseQuery, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			lib.Log.Error(err)
		}
	}(rows)

	var result []Package

	for rows.Next() {
		var pkg Package
		var dependsStr string
		var providersStr string
		var installedInt int

		if err = rows.Scan(
			&pkg.Name,
			&pkg.Section,
			&pkg.InstalledSize,
			&pkg.Maintainer,
			&pkg.Version,
			&pkg.VersionInstalled,
			&dependsStr,
			&providersStr,
			&pkg.Size,
			&pkg.Filename,
			&pkg.Description,
			&pkg.Changelog,
			&installedInt,
		); err != nil {
			return nil, fmt.Errorf("ошибка чтения данных о пакете: %w", err)
		}

		if providersStr != "" {
			pkg.Provides = strings.Split(providersStr, ",")
		} else {
			pkg.Provides = []string{}
		}

		if dependsStr != "" {
			pkg.Depends = strings.Split(dependsStr, ",")
		} else {
			pkg.Depends = []string{}
		}

		pkg.Installed = installedInt != 0
		result = append(result, pkg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %w", err)
	}

	return result, nil
}

// QueryHostImagePackages возвращает пакеты из таблицы host_image_packages
// с возможностью фильтрации и сортировкой
func (s *PackageDBService) QueryHostImagePackages(
	ctx context.Context,
	filters map[string]interface{},
	sortField, sortOrder string,
	limit, offset int64,
) ([]Package, error) {

	query := fmt.Sprintf(`
        SELECT 
            name,
            section,
            installed_size,
            maintainer,
            version,
            versionInstalled,
            depends,
            provides,
            size,
            filename,
            description,
            changelog,
            installed
        FROM %s
    `, s.tableName)

	var args []interface{}

	// Формируем WHERE-условие, если есть фильтры.
	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			// Если фильтруем по полю "installed", делаем особую логику
			if field == "installed" {
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
			} else if field == "provides" || field == "depends" {
				if strVal, ok := value.(string); ok {
					conditions = append(conditions, fmt.Sprintf("',' || %s || ',' LIKE ?", field))
					args = append(args, fmt.Sprintf("%%,%s,%%", strVal))
				} else {
					conditions = append(conditions, fmt.Sprintf("',' || %s || ',' LIKE ?", field))
					args = append(args, fmt.Sprintf("%%,%v,%%", value))
				}
			} else {
				if strVal, ok := value.(string); ok {
					conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
					args = append(args, fmt.Sprintf("%%%s%%", strVal))
				} else {
					conditions = append(conditions, fmt.Sprintf("%s = ?", field))
					args = append(args, value)
				}
			}
		}

		if len(conditions) > 0 {
			whereClause := strings.Join(conditions, " AND ")
			query += " WHERE " + whereClause
		}
	}

	// Добавляем сортировку, если указаны поле и порядок
	if sortField != "" {
		upperOrder := strings.ToUpper(sortOrder)
		if upperOrder != "ASC" && upperOrder != "DESC" {
			upperOrder = "ASC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", sortField, upperOrder)
	}

	// Добавляем LIMIT/OFFSET
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	// Выполняем запрос
	rows, err := s.dbConn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			lib.Log.Error(err)
		}
	}(rows)

	var result []Package

	for rows.Next() {
		var pkg Package
		var dependsStr string
		var providersStr string
		var installedInt int

		if err = rows.Scan(
			&pkg.Name,
			&pkg.Section,
			&pkg.InstalledSize,
			&pkg.Maintainer,
			&pkg.Version,
			&pkg.VersionInstalled,
			&dependsStr,
			&providersStr,
			&pkg.Size,
			&pkg.Filename,
			&pkg.Description,
			&pkg.Changelog,
			&installedInt,
		); err != nil {
			return nil, fmt.Errorf("ошибка чтения данных о пакете: %w", err)
		}

		if providersStr != "" {
			pkg.Provides = strings.Split(providersStr, ",")
		} else {
			pkg.Provides = []string{}
		}

		if dependsStr != "" {
			pkg.Depends = strings.Split(dependsStr, ",")
		} else {
			pkg.Depends = []string{}
		}

		pkg.Installed = installedInt != 0
		result = append(result, pkg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %w", err)
	}

	return result, nil
}

// CountHostImagePackages возвращает количество записей из таблицы host_image_packages
// с учётом переданных фильтров (строки => LIKE '%...%', для остальных типов "=").
func (s *PackageDBService) CountHostImagePackages(ctx context.Context, filters map[string]interface{}) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)

	var args []interface{}
	if len(filters) > 0 {
		var conditions []string
		for field, value := range filters {
			// Если фильтруем по полю "installed", делаем особую логику
			if field == "installed" {
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
			} else if field == "provides" || field == "depends" {
				if strVal, ok := value.(string); ok {
					conditions = append(conditions, fmt.Sprintf("',' || %s || ',' LIKE ?", field))
					args = append(args, fmt.Sprintf("%%,%s,%%", strVal))
				} else {
					conditions = append(conditions, fmt.Sprintf("',' || %s || ',' LIKE ?", field))
					args = append(args, fmt.Sprintf("%%,%v,%%", value))
				}
			} else {
				if strVal, ok := value.(string); ok {
					conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
					args = append(args, fmt.Sprintf("%%%s%%", strVal))
				} else {
					conditions = append(conditions, fmt.Sprintf("%s = ?", field))
					args = append(args, value)
				}
			}
		}

		if len(conditions) > 0 {
			whereClause := strings.Join(conditions, " AND ")
			query += " WHERE " + whereClause
		}
	}

	var totalCount int64
	if err := s.dbConn.QueryRowContext(ctx, query, args...).Scan(&totalCount); err != nil {
		return 0, fmt.Errorf("ошибка при подсчёте количества пакетов: %w", err)
	}

	return totalCount, nil
}

// PackageDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func (s *PackageDBService) PackageDatabaseExist(ctx context.Context) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)
	var count int
	err := s.dbConn.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return err
	}

	return nil
}
