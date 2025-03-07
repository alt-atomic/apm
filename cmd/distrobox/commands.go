package distrobox

import (
	"apm/cmd/common/reply"
	"apm/cmd/distrobox/api"
	"apm/cmd/distrobox/service"
	"apm/lib"
	"context"
	"fmt"
	"github.com/urfave/cli/v3"
	"strings"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func validateContainer(cmd *cli.Command) (string, reply.APIResponse, error) {
	var resp reply.APIResponse
	containerVal := cmd.String("container")
	errText := "необходимо указать название контейнера (--container | -c)"

	if strings.TrimSpace(containerVal) == "" {
		resp = reply.APIResponse{
			Data:  map[string]interface{}{"message": errText},
			Error: true,
		}
		return "", resp, fmt.Errorf(errText)
	}

	errContainer := service.ContainerDatabaseExist(containerVal)

	if errContainer != nil {
		osInfo, err := api.GetContainerOsInfo(containerVal)
		if err != nil {
			resp = reply.APIResponse{
				Data:  map[string]interface{}{"message": err.Error()},
				Error: true,
			}
			return "", resp, fmt.Errorf(errText)
		}

		_, err = service.UpdatePackages(osInfo)
		if err != nil {
			resp = reply.APIResponse{
				Data:  map[string]interface{}{"message": err.Error()},
				Error: true,
			}
			return "", resp, fmt.Errorf(errText)
		}
	}

	return containerVal, reply.APIResponse{}, nil
}

func pluralizePackage(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "пакет"
	} else if n%10 >= 2 && n%10 <= 4 && !(n%100 >= 12 && n%100 <= 14) {
		return "пакета"
	}
	return "пакетов"
}

func withGlobalWrapper(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		lib.Env.Format = cmd.String("format")
		lib.Env.Transaction = cmd.String("transaction")

		reply.CreateSpinner()
		return action(ctx, cmd)
	}
}

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "distrobox",
		Aliases: []string{"d"},
		Usage:   "Управление пакетами и контейнерами и контейнерами distrobox",
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: "Обновить и синхронизировать списки установленных пакетов с хостом",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Required: true,
						Aliases:  []string{"c"},
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					osInfo, err := api.GetContainerOsInfo(containerVal)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					packages, err := service.UpdatePackages(osInfo)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					resp = reply.APIResponse{
						Data: map[string]interface{}{
							"message":   "Список пакетов успешно обновлён",
							"container": osInfo,
							"count":     len(packages),
						},
						Error: false,
					}

					return reply.CliResponse(resp)
				}),
			},
			{
				Name:      "info",
				Usage:     "Информация о пакете",
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					packageName := cmd.Args().First()
					if cmd.Args().Len() == 0 || packageName == "" {
						return reply.CliResponse(newErrorResponse("необходимо указать название пакета, например info package"))
					}

					osInfo, err := api.GetContainerOsInfo(containerVal)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					packageInfo, err := service.GetInfoPackage(osInfo, packageName)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					return reply.CliResponse(reply.APIResponse{
						Data: map[string]interface{}{
							"message":     "Информация о пакете",
							"packageInfo": packageInfo,
						},
						Error: false,
					})
				}),
			},
			{
				Name:      "search",
				Usage:     "Поиск пакета по названию",
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					packageName := cmd.Args().First()
					if cmd.Args().Len() == 0 || packageName == "" {
						return reply.CliResponse(newErrorResponse("необходимо указать название пакета, например search package"))
					}

					osInfo, err := api.GetContainerOsInfo(containerVal)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					queryResult, err := service.GetPackageByName(osInfo, packageName)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					word := pluralizePackage(queryResult.TotalCount)
					msg := fmt.Sprintf("Найден %d %s\n", queryResult.TotalCount, word)

					// Формируем ответ
					resp = reply.APIResponse{
						Data: map[string]interface{}{
							"message":  msg,
							"packages": queryResult.Packages,
							"count":    queryResult.TotalCount,
						},
						Error: false,
					}

					return reply.CliResponse(resp)
				}),
			},
			{
				Name:  "list",
				Usage: "Построение запроса для получения списка пакетов",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
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
						Usage: "Название поля для фильтрации (например, packageName, version, manager)",
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
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					builder := service.PackageQueryBuilder{
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
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					// Вызываем функцию запроса пакетов
					queryResult, err := service.GetPackagesQuery(osInfo, builder)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					word := pluralizePackage(queryResult.TotalCount)
					msg := fmt.Sprintf("Найдено: %d %s\n", queryResult.TotalCount, word)

					// Формируем ответ
					resp = reply.APIResponse{
						Data: map[string]interface{}{
							"message":  msg,
							"packages": queryResult.Packages,
							"count":    queryResult.TotalCount,
						},
						Error: false,
					}

					return reply.CliResponse(resp)
				}),
			},
			{
				Name:      "install",
				Usage:     "Установить пакет",
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "export",
						Usage: "Экспортировать пакет",
						Value: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					osInfo, err := api.GetContainerOsInfo(containerVal)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					packageName := cmd.Args().First()
					if cmd.Args().Len() == 0 || packageName == "" {
						return reply.CliResponse(newErrorResponse("необходимо указать название пакета, например search package"))
					}

					packageInfo, err := service.GetInfoPackage(osInfo, packageName)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					if !packageInfo.Package.Installed {
						err = service.InstallPackage(osInfo, packageName)
						if err != nil {
							return reply.CliResponse(newErrorResponse(err.Error()))
						}

						packageInfo.Package.Installed = true
						service.UpdatePackageField(osInfo.ContainerName, packageName, "installed", true)
						packageInfo, _ = service.GetInfoPackage(osInfo, packageName)
					}

					if cmd.Bool("export") && !packageInfo.Package.Exporting {
						errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, false)
						if errExport != nil {
							return reply.CliResponse(newErrorResponse(errExport.Error()))
						}

						packageInfo.Package.Exporting = true
						service.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", true)
					}

					return reply.CliResponse(reply.APIResponse{
						Data: map[string]interface{}{
							"message": fmt.Sprintf("Пакет %s установлен", packageName),
							"package": packageInfo,
						},
						Error: false,
					})
				}),
			},
			{
				Name:      "remove",
				Usage:     "Удалить пакет",
				ArgsUsage: "package",
				Aliases:   []string{"rm"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "only-export",
						Usage: "Удалить только экспорт, оставить пакет в контейнере",
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					containerVal, resp, err := validateContainer(cmd)
					if err != nil {
						return reply.CliResponse(resp)
					}

					packageName := cmd.Args().First()
					if cmd.Args().Len() == 0 || packageName == "" {
						return reply.CliResponse(newErrorResponse("необходимо указать название пакета, например search package"))
					}

					osInfo, err := api.GetContainerOsInfo(containerVal)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					packageInfo, err := service.GetInfoPackage(osInfo, packageName)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					if packageInfo.Package.Exporting {
						errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, true)
						if errExport != nil {
							return reply.CliResponse(newErrorResponse(errExport.Error()))
						}

						packageInfo.Package.Exporting = false
						service.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", false)
					}

					if !cmd.Bool("only-export") && packageInfo.Package.Installed {
						err = service.RemovePackage(osInfo, packageName)
						if err != nil {
							return reply.CliResponse(newErrorResponse(err.Error()))
						}

						packageInfo.Package.Installed = false
						service.UpdatePackageField(osInfo.ContainerName, packageName, "installed", false)
					}

					return reply.CliResponse(reply.APIResponse{
						Data: map[string]interface{}{
							"message": fmt.Sprintf("Пакет %s удалён", packageName),
							"package": packageInfo,
						},
						Error: false,
					})
				}),
			},
			{
				Name:    "container",
				Usage:   "Модуль для работы с контейнерами",
				Aliases: []string{"c"},
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "Список контейнеров",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							containers, err := api.GetContainerList(true)
							if err != nil {
								return reply.CliResponse(newErrorResponse(err.Error()))
							}

							var names []string
							for _, c := range containers {
								names = append(names, c.ContainerName)
							}

							resp := reply.APIResponse{
								Data: map[string]interface{}{
									"containers": containers,
								},
								Error: false,
							}
							return reply.CliResponse(resp)
						}),
					},
					{
						Name:  "add",
						Usage: "Добавить контейнер",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    "Ссылка на образ. Необходимо указать",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    "Название контейнера. Необходимо указать",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "additional-packages",
								Usage: "Список пакетов для установки",
								Value: "zsh",
							},
							&cli.StringFlag{
								Name:  "init-hooks",
								Usage: "Вызов хука для выполнения команд",
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							imageVal := cmd.String("image")
							nameVal := cmd.String("name")
							addPkgVal := cmd.String("additional-packages")
							hookVal := cmd.String("init-hooks")

							if strings.TrimSpace(imageVal) == "" {
								return reply.CliResponse(newErrorResponse("Необходимо указать ссылку на образ (--image)"))
							}

							if strings.TrimSpace(nameVal) == "" {
								return reply.CliResponse(newErrorResponse("Необходимо указать название контейнера (--name)"))
							}

							result, err := api.CreateContainer(imageVal, nameVal, addPkgVal, hookVal)
							if err != nil {
								return reply.CliResponse(newErrorResponse(fmt.Sprintf("Ошибка создания контейнера: %v", err)))
							}

							resp := reply.APIResponse{
								Data: map[string]interface{}{
									"message":       fmt.Sprintf("Контейнер %s успешно создан", nameVal),
									"containerInfo": result,
								},
								Error: false,
							}
							return reply.CliResponse(resp)
						}),
					},
					{
						Name:    "remove",
						Usage:   "Удалить контейнер",
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "Название контейнера. Необходимо указать",
								Required: true,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							nameVal := cmd.String("name")
							if strings.TrimSpace(nameVal) == "" {
								return reply.CliResponse(newErrorResponse("Необходимо указать название контейнера (--name)"))
							}

							result, err := api.RemoveContainer(nameVal)
							if err != nil {
								return reply.CliResponse(newErrorResponse(fmt.Sprintf("Ошибка удаления контейнера: %v", err)))
							}

							resp := reply.APIResponse{
								Data: map[string]interface{}{
									"message":       fmt.Sprintf("Контейнер %s успешно удалён", nameVal),
									"containerInfo": result,
								},
								Error: false,
							}

							return reply.CliResponse(resp)
						}),
					},
				},
			},
		},
	}
}
