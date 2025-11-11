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
	"apm/internal/distrobox/service"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/akrylysov/pogreb"
)

// Service — сервис иконок
type Service struct {
	serviceDistroAPI *service.DistroAPIService
	dbConnKv         *pogreb.DB
	commandPrefix    string
}

// NewIconService — конструктор сервиса
func NewIconService(db *pogreb.DB, commandPrefix string) *Service {
	distroAPISvc := service.NewDistroAPIService(commandPrefix)
	return &Service{
		serviceDistroAPI: distroAPISvc,
		dbConnKv:         db,
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
	data, err := s.getIconFromDB(pkgName, container)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Icon for package %s not found: %v"), pkgName, err)
	}
	return data, nil
}

// ReloadIcons загружает и сохраняет иконки из SWCatalog в базу данных.
func (s *Service) ReloadIcons(ctx context.Context) error {
	containerList, err := s.serviceDistroAPI.GetContainerList(ctx, true)
	if err != nil {
		app.Log.Error(err.Error())
		containerList = nil
		//return err
	}

	// Вызываем сборщик мусора после загрузки
	defer runtime.GC()

	// Обработка системных пакетов
	systemPackages, err := s.getPackages(ctx, "")
	if err != nil {
		return err
	}
	for _, pkgIcon := range systemPackages {
		if exists, _ := s.packageIconExists(pkgIcon.Name, pkgIcon.Container); !exists {
			if err := s.saveIconToDB(pkgIcon.Name, pkgIcon.Container, pkgIcon.Icon); err != nil {
				app.Log.Error(fmt.Sprintf(app.T_("Error saving icon %s: %v"), pkgIcon.Name, err))
			}
		}
	}

	// Обработка иконок для каждого контейнера
	for _, distroContainer := range containerList {
		if distroContainer.OS == "Arch" {
			distroPackages, err := s.getPackages(ctx, distroContainer.ContainerName)
			if err != nil {
				app.Log.Error(err)
				continue
			}
			for _, pkgIcon := range distroPackages {
				if exists, _ := s.packageIconExists(pkgIcon.Name, pkgIcon.Container); !exists {
					if err = s.saveIconToDB(pkgIcon.Name, pkgIcon.Container, pkgIcon.Icon); err != nil {
						app.Log.Error(fmt.Sprintf(app.T_("Error saving icon %s: %v"), pkgIcon.Name, err))
					}
				}
			}
		}
	}

	// Вывод статистики из БД
	count, totalSize, err := s.computeDBIconsStats()
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

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, 20)
		mu  sync.Mutex
	)

	for _, pkgSwIcon := range packageSwCatIcons {
		// Если иконка уже сохранена в БД, пропускаем её
		exists, err := s.packageIconExists(pkgSwIcon.PkgName, container)
		if err != nil {
			app.Log.Error(app.T_("Error checking the existence of the icon in the database: "), err)
			continue
		}
		if exists {
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

// SaveIconToDB сохраняет иконку в базу данных Pogreb.
// Ключ формируется по схеме: "icon:<container>:<pkgName>"
func (s *Service) saveIconToDB(pkgName, container string, iconData []byte) error {
	key := []byte(fmt.Sprintf("icon:%s:%s", container, pkgName))
	// iconData уже сжат, поэтому записываем его напрямую.
	if err := s.dbConnKv.Put(key, iconData); err != nil {
		return fmt.Errorf(app.T_("Error writing icon %s to the database: %v"), pkgName, err)
	}
	return nil
}

// GetIconFromDB извлекает и возвращает распакованную иконку из базы данных.
func (s *Service) getIconFromDB(pkgName, container string) ([]byte, error) {
	// Формируем ключ с учетом переданного контейнера
	key := []byte(fmt.Sprintf("icon:%s:%s", container, pkgName))
	compressedIcon, err := s.dbConnKv.Get(key)
	// Если не найдено и контейнер указан, пробуем без контейнера
	if (err != nil || len(compressedIcon) == 0) && container != "" {
		fallbackKey := []byte(fmt.Sprintf("icon::%s", pkgName))
		compressedIcon, err = s.dbConnKv.Get(fallbackKey)
	}
	if err != nil || len(compressedIcon) == 0 {
		return nil, fmt.Errorf(app.T_("Icon %s not found"), pkgName)
	}
	decompressed, err := decompressIcon(compressedIcon)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error unpacking icon %s: %v"), pkgName, err)
	}
	return decompressed, nil
}

// packageIconExists проверяет наличие иконки в базе по ключу "icon:<container>:<pkgName>".
func (s *Service) packageIconExists(pkgName, container string) (bool, error) {
	key := []byte(fmt.Sprintf("icon:%s:%s", container, pkgName))
	data, err := s.dbConnKv.Get(key)
	if err != nil {
		return false, err
	}
	return data != nil && len(data) > 0, nil
}

// computeDBIconsStats вычисляет количество и общий размер всех иконок, сохранённых в БД.
func (s *Service) computeDBIconsStats() (int, int, error) {
	it := s.dbConnKv.Items()
	count := 0
	totalSize := 0
	for {
		key, value, err := it.Next()
		if errors.Is(pogreb.ErrIterationDone, err) {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		if bytes.HasPrefix(key, []byte("icon:")) {
			count++
			totalSize += len(value)
		}
	}
	return count, totalSize, nil
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
