package icon

import (
	"apm/cmd/distrobox/service"
	"apm/lib"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// Service — сервис иконок
type Service struct {
	serviceDistroAPI *service.DistroAPIService
}

// NewIconService — конструктор сервиса
func NewIconService() *Service {
	distroAPISvc := service.NewDistroAPIService()
	return &Service{
		serviceDistroAPI: distroAPISvc,
	}
}

var (
	AllPackageIcons []PackageIcon
)

type PackageIcon struct {
	Name      string `json:"name"`
	Icon      []byte `json:"icon"`
	Container string `json:"container"`
}

// GetIcon возвращает распакованную иконку по имени.
func (s *Service) GetIcon(name string) ([]byte, error) {
	for _, icon := range AllPackageIcons {
		if icon.Name == name {
			decompressed, err := decompressIcon(icon.Icon)
			if err != nil {
				return nil, fmt.Errorf("ошибка распаковки иконки %s: %v", name, err)
			}
			return decompressed, nil
		}
	}

	return nil, fmt.Errorf("иконка для пакета %s не найдена", name)
}

func (s *Service) ReloadIcons(ctx context.Context) error {
	containerList, err := s.serviceDistroAPI.GetContainerList(ctx, true)
	if err != nil {
		return err
	}

	defer runtime.GC()

	AllPackageIcons = make([]PackageIcon, 0)

	systemPackages, errSystem := s.getPackages("")
	if errSystem != nil {
		return err
	}

	AllPackageIcons = append(AllPackageIcons, systemPackages...)

	for _, distroContainer := range containerList {
		if distroContainer.OS == "Arch" {
			distroPackages, errDistro := s.getPackages(distroContainer.ContainerName)
			if errDistro != nil {
				lib.Log.Error(errDistro)
			} else {
				AllPackageIcons = append(AllPackageIcons, distroPackages...)
			}
		}
	}

	// Подсчитываем общий размер всех иконок.
	var totalSize int
	for _, icon := range AllPackageIcons {
		totalSize += len(icon.Icon)
	}

	lib.Log.Debugf("Общее количество иконок: %d, общий размер AllPackageIcons: %d байт\n",
		len(AllPackageIcons), totalSize)
	// Поиск и вывод дубликатов имен.
	//duplicates := findDuplicateNames(AllPackageIcons)
	//if len(duplicates) > 0 {
	//	fmt.Println("Найдены дубликаты имён:")
	//	for _, name := range duplicates {
	//		fmt.Println(name)
	//	}
	//} else {
	//	fmt.Println("Дубликатов имён не найдено.")
	//}

	return nil
}

func (s *Service) getPackages(container string) ([]PackageIcon, error) {
	var packageIcons []PackageIcon
	systemSwCatService := NewSwCatIconService("/usr/share/swcatalog/xml", container)

	packageSwCatIcons, err := systemSwCatService.LoadSWCatalogs()
	if err != nil {
		return []PackageIcon{}, err
	}

	var cachedBase, stockBase string
	var cleanup func()

	// ищем внутри контейнера, stockBase там вероятно будет пустой
	if container != "" {
		cachedBase, stockBase, cleanup, err = systemSwCatService.prepareTempIconDirs("/usr/share/swcatalog/icons", "")
		if err != nil {
			return []PackageIcon{}, fmt.Errorf("ошибка подготовки временных каталогов: %v", err)
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

	// Параллельно обрабатываем каждый пакет.
	for _, pkgSwIcon := range packageSwCatIcons {
		if packageIconExists(pkgSwIcon.PkgName) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(pkgSwIcon PackageIconsSwCat) {
			defer wg.Done()
			rawIcon, errFind := systemSwCatService.getIconFromPackage(pkgSwIcon, cachedBase, stockBase)
			if errFind != nil {
				<-sem
				return
			}

			// Сжимаем иконку перед сохранением.
			compressedIcon, err := compressIcon(rawIcon)
			if err != nil {
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

// packageIconExists проверяет, есть ли уже иконка с данным именем в AllPackageIcons.
func packageIconExists(pkgName string) bool {
	for _, icon := range AllPackageIcons {
		if icon.Name == pkgName {
			return true
		}
	}
	return false
}

// compressIcon сжимает переданный срез байт с помощью gzip.
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

// decompressIcon распаковывает сжатые данные с помощью gzip.
func decompressIcon(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func(reader *gzip.Reader) {
		err = reader.Close()
		if err != nil {
			lib.Log.Error(err)
		}
	}(reader)

	return io.ReadAll(reader)
}

// findDuplicateNames проходит по срезу icons и возвращает имена, встречающиеся более одного раза.
func findDuplicateNames(icons []PackageIcon) []string {
	nameCounts := make(map[string]int)
	var duplicates []string

	for _, icon := range icons {
		nameCounts[icon.Name]++
	}

	for name, count := range nameCounts {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}
	return duplicates
}
