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
	"apm/internal/common/sandbox"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// Service предоставляет сервис для работы с иконками.
type Service struct {
	serviceDistroAPI *sandbox.DistroAPIService
	dbService        *DBService
	commandPrefix    string
}

// NewIconService создаёт новый сервис для работы с иконками.
func NewIconService(dbManager app.DatabaseManager, commandPrefix string) *Service {
	distroAPISvc := sandbox.NewDistroAPIService(commandPrefix)
	iconDB := NewIconDBService(dbManager)

	return &Service{
		serviceDistroAPI: distroAPISvc,
		dbService:        iconDB,
		commandPrefix:    commandPrefix,
	}
}

// PackageIcon описывает иконку пакета.
// При сохранении в БД данные уже сжаты.
type PackageIcon struct {
	Name      string `json:"name"`
	Icon      []byte `json:"icon"`
	Container string `json:"container"`
}

// GetIcon возвращает распакованную иконку для указанного пакета из базы данных.
func (s *Service) GetIcon(pkgName, container string) ([]byte, error) {
	compressedIcon, err := s.dbService.GetIcon(pkgName, container)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Icon for package %s not found: %v"), pkgName, err)
	}

	decompressed, err := decompressIcon(compressedIcon)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error unpacking icon %s: %v"), pkgName, err)
	}
	return decompressed, nil
}

// ReloadIcons загружает и сохраняет иконки из SWCatalog в базу данных.
func (s *Service) ReloadIcons(ctx context.Context) error {
	containerList, err := s.serviceDistroAPI.GetContainerList(ctx, true)
	if err != nil {
		app.Log.Error(err.Error())
		containerList = nil
	}

	// Вызываем сборщик мусора после загрузки
	defer runtime.GC()

	// Обработка системных пакетов
	systemPackages, err := s.getPackages(ctx, "")
	if err != nil {
		return err
	}
	if err = s.saveNewIcons(systemPackages); err != nil {
		app.Log.Error(fmt.Sprintf(app.T_("Error saving icon batch: %v"), err))
	}

	// Обработка иконок для каждого контейнера
	for _, distroContainer := range containerList {
		if distroContainer.OS == "Arch" {
			distroPackages, err := s.getPackages(ctx, distroContainer.ContainerName)
			if err != nil {
				app.Log.Error(err)
				continue
			}
			if err = s.saveNewIcons(distroPackages); err != nil {
				app.Log.Error(fmt.Sprintf(app.T_("Error saving icon batch: %v"), err))
			}
		}
	}

	// Вывод статистики из БД
	count, totalSize, err := s.dbService.GetStats()
	if err != nil {
		app.Log.Error(app.T_("Error calculating icon statistics: "), err)
	} else {
		app.Log.Debugf(app.T_("Total number of icons in the database: %d, total size: %d bytes"), count, totalSize)
	}
	return nil
}

// getPackages получает иконки из SWCatalog для указанного контейнера.
func (s *Service) getPackages(ctx context.Context, container string) ([]PackageIcon, error) {
	var packageIcons []PackageIcon
	systemSwCatService := NewSwCatIconService("/usr/share/swcatalog/xml", container, s.commandPrefix)

	packageSwCatIcons, err := systemSwCatService.LoadSWCatalogs(ctx)
	if err != nil {
		return nil, err
	}

	var cachedBase, stockBase string
	var cleanup func()
	if container != "" {
		cachedBase, stockBase, cleanup, err = systemSwCatService.prepareTempIconDirs(ctx, "/usr/share/swcatalog/icons", "")
		if err != nil {
			return nil, fmt.Errorf(app.T_("Error preparing temporary directories: %v"), err)
		}
		defer cleanup()
	} else {
		cachedBase = "/usr/share/swcatalog/icons"
		stockBase = "/usr/share/icons/hicolor/128x128/apps"
	}

	// Загружаем все существующие иконки для контейнера одним запросом
	existingIcons, err := s.dbService.GetExistingPackages(container)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error checking the existence of the icon in the database: %v"), err)
	}

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, 20)
		mu  sync.Mutex
	)

	for _, pkgSwIcon := range packageSwCatIcons {
		// Если иконка уже сохранена в БД, пропускаем её
		if existingIcons[pkgSwIcon.PkgName] {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(pkgSwIcon PackageIconsSwCat) {
			defer wg.Done()
			rawIcon, errFind := systemSwCatService.getIconFromPackage(pkgSwIcon, cachedBase, stockBase)
			if errFind != nil {
				app.Log.Debugf(app.T_("Error retrieving icon: %s"), errFind.Error())
				<-sem
				return
			}
			compressedIcon, err := compressIcon(rawIcon)
			if err != nil {
				app.Log.Error(app.T_("Error compressing the icon: "), err)
				<-sem
				return
			}
			mu.Lock()
			packageIcons = append(packageIcons, PackageIcon{
				Name:      pkgSwIcon.PkgName,
				Icon:      compressedIcon,
				Container: container,
			})
			mu.Unlock()
			<-sem
		}(pkgSwIcon)
	}
	wg.Wait()

	return packageIcons, nil
}

// saveNewIcons сохраняет иконки пакетов "батчем"
func (s *Service) saveNewIcons(icons []PackageIcon) error {
	if len(icons) == 0 {
		return nil
	}
	dbIcons := make([]DBIcon, 0, len(icons))
	for _, ic := range icons {
		dbIcons = append(dbIcons, DBIcon{
			Package:   ic.Name,
			Container: ic.Container,
			Icon:      ic.Icon,
		})
	}
	return s.dbService.SaveIconsBatch(dbIcons)
}

// compressIcon сжимает данные с помощью gzip.
func compressIcon(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressIcon распаковывает данные, сжатые gzip.
func decompressIcon(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()
	return io.ReadAll(reader)
}
