package distrobox

import (
	"apm/cmd/distrobox/api"
	"apm/cmd/distrobox/os"
	"apm/logger"
	"context"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v3"
	"strconv"
	"strings"
)

// APIResponse описывает формат ответа
type APIResponse struct {
	Data  interface{} `json:"data"`
	Error bool        `json:"error"`
}

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) APIResponse {
	logger.Log.Error(message)

	return APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func packageInfoPlainText(packageInfo os.PackageInfo) string {
	installedStr := "нет"
	if packageInfo.Installed {
		installedStr = "да"
	}
	exportedStr := "нет"
	if packageInfo.Exporting {
		exportedStr = "да"
	}

	msg := fmt.Sprintf(
		"\nПакет:\n"+
			"  Название:              %s\n"+
			"  Описание:              %s\n"+
			"  Версия:                %s\n"+
			"  Установлен:            %s\n"+
			"  Экспортирован:         %s\n",
		packageInfo.PackageName,
		packageInfo.Description,
		packageInfo.Version,
		installedStr,
		exportedStr,
	)

	return msg
}

func validateContainer(cmd *cli.Command) (string, APIResponse, error) {
	var resp APIResponse
	containerVal := cmd.String("container")
	errText := "необходимо указать название контейнера (--container | -c)"

	if strings.TrimSpace(containerVal) == "" {
		resp = APIResponse{
			Data:  map[string]interface{}{"message": errText},
			Error: true,
		}
		return "", resp, fmt.Errorf(errText)
	}

	errContainer := os.ContainerDatabaseExist(containerVal)

	if errContainer != nil {
		osInfo, err := api.GetContainerOsInfo(containerVal)
		if err != nil {
			resp = APIResponse{
				Data:  map[string]interface{}{"message": err.Error()},
				Error: true,
			}
			return "", resp, fmt.Errorf(errText)
		}

		_, err = os.UpdatePackages(osInfo)
		if err != nil {
			resp = APIResponse{
				Data:  map[string]interface{}{"message": err.Error()},
				Error: true,
			}
			return "", resp, fmt.Errorf(errText)
		}
	}

	return containerVal, APIResponse{}, nil
}

// response анализирует формат и выводит данные в соответствии с ним.
func response(cmd *cli.Command, resp APIResponse) error {
	formatVal := cmd.Root().Value("format")
	format, ok := formatVal.(string)
	if !ok || format == "" {
		format = "text"
	}

	switch format {
	case "json":
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	default:
		var message string
		switch data := resp.Data.(type) {
		case map[string]interface{}:
			if m, ok := data["message"]; ok {
				if str, ok := m.(string); ok {
					message = str
				} else {
					message = fmt.Sprintf("%v", m)
				}
			}
		case map[string]string:
			message = data["message"]
		case string:
			message = data
		default:
			message = fmt.Sprintf("%v", resp.Data)
		}
		fmt.Println(message)
	}

	return nil
}

func pluralizePackage(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "пакет"
	} else if n%10 >= 2 && n%10 <= 4 && !(n%100 >= 12 && n%100 <= 14) {
		return "пакета"
	}
	return "пакетов"
}

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "distrobox",
		Aliases: []string{"d"},
		Usage:   "Управление контейнерами и пакетами",
		Commands: []*cli.Command{
			{
				Name:  "package",
				Usage: "Модуль для работы с пакетами",
				Commands: []*cli.Command{
					{
						Name:  "update",
						Usage: "Обновить и синхронизировать списки установленных пакетов с хостом",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							packages, err := os.UpdatePackages(osInfo)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							resp = APIResponse{
								Data:  map[string]interface{}{"container": osInfo, "message": "Список пакетов успешно обновлён, пакетов всего: " + strconv.Itoa(len(packages)), "countPackage": len(packages)},
								Error: false,
							}

							return response(cmd, resp)
						},
					},
					{
						Name:  "info",
						Usage: "Информация о пакете",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							packageName := cmd.Args().First()
							if cmd.Args().Len() == 0 || packageName == "" {
								resp = APIResponse{
									Data:  map[string]string{"message": "Необходимо указать название пакета"},
									Error: true,
								}
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							packageInfo, err := os.GetInfoPackage(osInfo, packageName)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							return response(cmd, APIResponse{
								Data: map[string]interface{}{
									"message": packageInfoPlainText(packageInfo.PackageInfo),
									"package": packageInfo,
								},
								Error: true,
							})
						},
					},
					{
						Name:  "search",
						Usage: "Поиск пакета по названию",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							packageName := cmd.Args().First()
							if cmd.Args().Len() == 0 || packageName == "" {
								return response(cmd, newErrorResponse("Необходимо указать название пакета"))
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							queryResult, err := os.GetPackageByName(osInfo, packageName)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							word := pluralizePackage(queryResult.TotalCount)
							msg := fmt.Sprintf("Найдено: %d %s\n", queryResult.TotalCount, word)
							for _, pkg := range queryResult.Packages {
								msg += packageInfoPlainText(pkg)
							}

							// Формируем ответ
							resp = APIResponse{
								Data: map[string]interface{}{
									"message":  msg,
									"packages": queryResult.Packages,
								},
								Error: false,
							}

							return response(cmd, resp)
						},
					},
					{
						Name:  "list",
						Usage: "Построение запроса для получения списка пакетов",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
							&cli.StringFlag{
								Name:  "sort",
								Usage: "Поле для сортировки (например, packageName, version)",
							},
							&cli.StringFlag{
								Name:  "order",
								Usage: "Порядок сортировки: ASC или DESC",
								Value: "ASC",
							},
							&cli.IntFlag{
								Name:  "limit",
								Usage: "Лимит выборки",
								Value: 10,
							},
							&cli.IntFlag{
								Name:  "offset",
								Usage: "Смещение выборки",
								Value: 0,
							},
							&cli.StringFlag{
								Name:  "filter-field",
								Usage: "Имя поля для фильтрации (например, packageName, version, manager)",
							},
							&cli.StringFlag{
								Name:  "filter-value",
								Usage: "Значение для фильтрации по указанному полю",
							},
							&cli.BoolFlag{
								Name:  "force-update",
								Usage: "Принудительно обновить все пакеты перед запросом",
								Value: false,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							builder := os.PackageQueryBuilder{
								ForceUpdate: cmd.Bool("force-update"),
								Limit:       cmd.Int("limit"),
								Offset:      cmd.Int("offset"),
								SortField:   cmd.String("sort"),
								SortOrder:   cmd.String("order"),
								Filters:     make(map[string]interface{}),
							}

							filterField := cmd.String("filter-field")
							filterValue := cmd.String("filter-value")
							if strings.TrimSpace(filterField) != "" && strings.TrimSpace(filterValue) != "" {
								builder.Filters[filterField] = filterValue
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							// Вызываем функцию запроса пакетов
							queryResult, err := os.GetPackagesQuery(osInfo, builder)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							word := pluralizePackage(queryResult.TotalCount)
							msg := fmt.Sprintf("Найдено: %d %s\n", queryResult.TotalCount, word)
							for _, pkg := range queryResult.Packages {
								msg += packageInfoPlainText(pkg)
							}

							// Формируем ответ
							resp = APIResponse{
								Data: map[string]interface{}{
									"message":  msg,
									"packages": queryResult.Packages,
									"total":    queryResult.TotalCount,
								},
								Error: false,
							}

							return response(cmd, resp)
						},
					},
					{
						Name:  "install",
						Usage: "Установить пакет",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
							&cli.BoolFlag{
								Name:  "export",
								Usage: "Экспортировать пакет",
								Value: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							packageName := cmd.Args().First()
							if cmd.Args().Len() == 0 || packageName == "" {
								return response(cmd, newErrorResponse("Необходимо указать название пакета"))
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							packageInfo, err := os.GetInfoPackage(osInfo, packageName)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							if !packageInfo.PackageInfo.Installed {
								err = os.InstallPackage(osInfo, packageName)
								if err != nil {
									return response(cmd, newErrorResponse(err.Error()))
								}

								packageInfo.PackageInfo.Installed = true
								os.UpdatePackageField(osInfo.ContainerName, packageName, "installed", true)
								packageInfo, _ = os.GetInfoPackage(osInfo, packageName)
							}

							if cmd.Bool("export") && !packageInfo.PackageInfo.Exporting {
								errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, false)
								if errExport != nil {
									return response(cmd, newErrorResponse(errExport.Error()))
								}

								packageInfo.PackageInfo.Exporting = true
								os.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", true)
							}

							return response(cmd, APIResponse{
								Data: map[string]interface{}{
									"message": packageInfoPlainText(packageInfo.PackageInfo),
									"package": packageInfo,
								},
								Error: false,
							})
						},
					},
					{
						Name:  "rm",
						Usage: "Удалить пакет",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "container",
								Usage:   "Название контейнера",
								Aliases: []string{"c"},
							},
							&cli.BoolFlag{
								Name:  "only-export",
								Usage: "Удалить только экспорт, оставить пакет в контейнере",
								Value: false,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containerVal, resp, err := validateContainer(cmd)
							if err != nil {
								return response(cmd, resp)
							}

							packageName := cmd.Args().First()
							if cmd.Args().Len() == 0 || packageName == "" {
								return response(cmd, newErrorResponse("Необходимо указать название пакета"))
							}

							osInfo, err := api.GetContainerOsInfo(containerVal)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							packageInfo, err := os.GetInfoPackage(osInfo, packageName)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							if packageInfo.PackageInfo.Exporting {
								errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, true)
								if errExport != nil {
									return response(cmd, newErrorResponse(errExport.Error()))
								}

								packageInfo.PackageInfo.Exporting = false
								os.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", false)
							}

							if !cmd.Bool("only-export") && packageInfo.PackageInfo.Installed {
								err = os.RemovePackage(osInfo, packageName)
								if err != nil {
									return response(cmd, newErrorResponse(err.Error()))
								}

								packageInfo.PackageInfo.Installed = false
								os.UpdatePackageField(osInfo.ContainerName, packageName, "installed", false)
							}

							return response(cmd, APIResponse{
								Data: map[string]interface{}{
									"message": packageInfoPlainText(packageInfo.PackageInfo),
									"package": packageInfo,
								},
								Error: false,
							})
						},
					},
				},
			},
			{
				Name:  "container",
				Usage: "Модуль для работы с контейнерами",
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "Список контейнеров",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							containers, err := api.GetContainerList(true)
							if err != nil {
								return response(cmd, newErrorResponse(err.Error()))
							}

							var names []string
							for _, c := range containers {
								names = append(names, c.ContainerName)
							}

							resp := APIResponse{
								Data: map[string]interface{}{
									"message":    "Список контейнеров: " + strings.Join(names, ", "),
									"containers": containers,
								},
								Error: false,
							}
							return response(cmd, resp)
						},
					},
					{
						Name:  "add",
						Usage: "Добавить контейнер",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "image",
								Usage: "Ссылка на образ",
							},
							&cli.StringFlag{
								Name:  "name",
								Usage: "Название контейнера",
							},
							&cli.StringFlag{
								Name:  "additional-packages",
								Usage: "Список пакетов для установки",
								Value: "zsh",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							var resp APIResponse
							imageVal := cmd.String("image")
							nameVal := cmd.String("name")
							addPkgVal := cmd.String("additional-packages")

							if strings.TrimSpace(imageVal) == "" {
								return response(cmd, newErrorResponse("Необходимо указать ссылку на образ (--image)"))
							}

							if strings.TrimSpace(nameVal) == "" {
								return response(cmd, newErrorResponse("Необходимо указать название контейнера (--name)"))
							}

							result, err := api.CreateContainer(imageVal, nameVal, addPkgVal)
							if err != nil {
								return response(cmd, newErrorResponse(fmt.Sprintf("Ошибка создания контейнера: %v", err)))
							}

							resp = APIResponse{
								Data: map[string]interface{}{
									"message":       fmt.Sprintf("Контейнер %s успешно создан", nameVal),
									"containerInfo": result,
								},
								Error: false,
							}
							return response(cmd, resp)
						},
					},
					{
						Name:  "rm",
						Usage: "Удалить контейнер",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "name",
								Usage: "Название контейнера",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							var resp APIResponse

							//containerName := cmd.Args().First()
							//if cmd.Args().Len() == 0 || containerName == "" {
							//	resp = APIResponse{
							//		Data:  map[string]string{"message": "Контейнер не указан"},
							//		Error: true,
							//	}
							//}
							nameVal := cmd.String("name")
							if strings.TrimSpace(nameVal) == "" {
								return response(cmd, newErrorResponse("Необходимо указать название контейнера (--name)"))
							}

							result, err := api.RemoveContainer(nameVal)
							if err != nil {
								return response(cmd, newErrorResponse(fmt.Sprintf("Ошибка удаления контейнера: %v", err)))
							}

							resp = APIResponse{
								Data: map[string]interface{}{
									"message":       fmt.Sprintf("Контейнер %s успешно удалён", nameVal),
									"containerInfo": result,
								},
								Error: false,
							}

							return response(cmd, resp)
						},
					},
				},
			},
		},
	}
}
