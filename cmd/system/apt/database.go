package apt

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"fmt"
	"strings"
)

// SavePackagesToDB сохраняет список пакетов
func SavePackagesToDB(ctx context.Context, packages []Package) error {
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
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
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
				dependsStr,
				pkg.Size,
				pkg.Filename,
				pkg.Description,
				pkg.Changelog,
				installed,
			)
		}

		query := fmt.Sprintf("INSERT INTO %s (name, section, installed_size, maintainer, version, depends, size, filename, description, changelog, installed) VALUES %s",
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

// GetPackageByName возвращает запись пакета с указанным именем из таблицы host_image_packages.
func GetPackageByName(ctx context.Context, packageName string) (Package, error) {
	query := `
		SELECT name, section, installed_size, maintainer, version, depends, size, filename, description, changelog, installed 
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
