package apt

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"fmt"
	"strings"
	"sync"
)

// syncDBMutex защищает операции синхронизации базы пакетов.
var syncDBMutex sync.Mutex

// SavePackagesToDB сохраняет список пакетов
func SavePackagesToDB(ctx context.Context, packages []Package) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	tableName := "host_image_packages"

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
		size INTEGER,
		filename TEXT,
		description TEXT,
		changelog TEXT,
		installed INTEGER
	)`, tableName)
	if _, err := lib.DB.Exec(createQuery); err != nil {
		return fmt.Errorf("ошибка создания таблицы: %w", err)
	}

	// Очищаем таблицу.
	deleteQuery := fmt.Sprintf("DELETE FROM %s", tableName)
	if _, err := lib.DB.Exec(deleteQuery); err != nil {
		return fmt.Errorf("ошибка очистки таблицы: %w", err)
	}

	// Начинаем транзакцию.
	tx, err := lib.DB.Begin()
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
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			// Преобразуем срез зависимостей в одну строку, разделённую запятыми.
			dependsStr := strings.Join(pkg.Depends, ",")
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
				pkg.Size,
				pkg.Filename,
				pkg.Description,
				pkg.Changelog,
				installed,
			)
		}

		query := fmt.Sprintf("INSERT INTO %s (name, section, installed_size, maintainer, version, versionInstalled, depends, size, filename, description, changelog, installed) VALUES %s",
			tableName, strings.Join(placeholders, ","))
		if _, err := tx.Exec(query, args...); err != nil {
			tx.Rollback()
			return fmt.Errorf("ошибка вставки пакетов: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

// GetPackageByName возвращает запись пакета
func GetPackageByName(ctx context.Context, packageName string) (Package, error) {
	query := `
		SELECT name, section, installed_size, maintainer, version, versionInstalled, depends, size, filename, description, changelog, installed 
		FROM host_image_packages 
		WHERE name = ?`

	var pkg Package
	var dependsStr string
	var installed int

	err := lib.DB.QueryRowContext(ctx, query, packageName).Scan(
		&pkg.Name,
		&pkg.Section,
		&pkg.InstalledSize,
		&pkg.Maintainer,
		&pkg.Version,
		&pkg.VersionInstalled,
		&dependsStr,
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

	pkg.Installed = installed != 0

	return pkg, nil
}

// SyncPackageInstallationInfo синхронизирует базу пакетов с результатом выполнения apt.GetInstalledPackages().
func SyncPackageInstallationInfo(ctx context.Context, installedPackages map[string]string) error {
	syncDBMutex.Lock()
	defer syncDBMutex.Unlock()

	tx, err := lib.DB.BeginTx(ctx, nil)
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
	if _, err := tx.ExecContext(ctx, createTempTableQuery); err != nil {
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
		if _, err := tx.ExecContext(ctx, insertQuery, args...); err != nil {
			return fmt.Errorf("ошибка пакетной вставки во временную таблицу: %w", err)
		}
	}

	// 3. Обновляем таблицу host_image_packages на основе временной таблицы
	updateQuery := `
        UPDATE host_image_packages
        SET 
            installed = CASE 
                WHEN EXISTS (SELECT 1 FROM tmp_installed t WHERE t.name = host_image_packages.name) THEN 1 
                ELSE 0 
            END,
            versionInstalled = COALESCE(
                (SELECT t.version FROM tmp_installed t WHERE t.name = host_image_packages.name), 
                ''
            )
    `
	if _, err := tx.ExecContext(ctx, updateQuery); err != nil {
		return fmt.Errorf("ошибка обновления пакетов: %w", err)
	}

	// 4. Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

// SearchPackagesByName ищет пакеты в таблице по части названия
func SearchPackagesByName(ctx context.Context, namePart string) ([]Package, error) {
	tableName := "host_image_packages"
	query := fmt.Sprintf(`
		SELECT 
			name, 
			section, 
			installed_size, 
			maintainer, 
			version, 
			versionInstalled, 
			depends, 
			size, 
			filename, 
			description, 
			changelog, 
			installed 
		FROM %s
		WHERE name LIKE ?
	`, tableName)

	// Подготавливаем шаблон для поиска, например "%имя%"
	searchPattern := "%" + namePart + "%"

	rows, err := lib.DB.QueryContext(ctx, query, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer rows.Close()

	var result []Package

	for rows.Next() {
		var pkg Package
		var dependsStr string
		var installed int

		if err := rows.Scan(
			&pkg.Name,
			&pkg.Section,
			&pkg.InstalledSize,
			&pkg.Maintainer,
			&pkg.Version,
			&pkg.VersionInstalled,
			&dependsStr,
			&pkg.Size,
			&pkg.Filename,
			&pkg.Description,
			&pkg.Changelog,
			&installed,
		); err != nil {
			return nil, fmt.Errorf("ошибка чтения данных о пакете: %w", err)
		}

		if dependsStr != "" {
			pkg.Depends = strings.Split(dependsStr, ",")
		} else {
			pkg.Depends = []string{}
		}

		pkg.Installed = installed != 0
		result = append(result, pkg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %w", err)
	}

	return result, nil
}

// PackageDatabaseExist проверяет, существует ли таблица и содержит ли она хотя бы одну запись.
func PackageDatabaseExist(ctx context.Context) error {
	tableName := "host_image_packages"
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	var count int
	err := lib.DB.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return err
	}

	return nil
}
