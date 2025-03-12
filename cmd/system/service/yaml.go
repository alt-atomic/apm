package service

import (
	"apm/lib"
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"time"
)

// Config описывает структуру конфигурационного файла.
type Config struct {
	Image    string `yaml:"image" json:"image"`
	Packages struct {
		Install []string `yaml:"install" json:"install"`
		Remove  []string `yaml:"remove" json:"remove"`
	} `yaml:"packages" json:"packages"`
	Commands []string `yaml:"commands" json:"commands"`
}

// ParseConfig читает YAML-конфигурацию из файла
func ParseConfig() (Config, error) {
	pathConfig := lib.Env.PathImageFile
	if len(pathConfig) == 0 {
		return Config{}, fmt.Errorf("необходимо указать pathImageFile в конфигурционном файле")
	}

	data, err := os.ReadFile(pathConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return generateFile(pathConfig)
		}
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	if len(cfg.Image) == 0 {
		return Config{}, fmt.Errorf("необходимо указать image в конфигурционном файле")
	}

	return cfg, nil
}

// GenerateDockerfile генерирует содержимое Dockerfile, формируя apt-get команды с модификаторами для пакетов.
func (c *Config) GenerateDockerfile() error {
	errCommands := c.CheckCommands()
	if errCommands != nil {
		return errCommands
	}

	// Формирование базовой apt-get команды.
	aptCmd := "apt-get update"

	// Формирование списка пакетов с суффиксами: + для установки и - для удаления.
	var pkgs []string
	uniqueInstall := uniqueStrings(c.Packages.Install)
	uniqueRemove := uniqueStrings(c.Packages.Remove)

	for _, pkg := range uniqueInstall {
		pkgs = append(pkgs, pkg+"+")
	}
	for _, pkg := range uniqueRemove {
		pkgs = append(pkgs, pkg+"-")
	}
	if len(pkgs) > 0 {
		aptCmd += " && apt-get -y install " + strings.Join(pkgs, " ")
	}

	// Формирование Dockerfile.
	var dockerfileLines []string
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("FROM \"%s\"", c.Image))
	// Разбиваем apt-get команду по строкам.
	aptLines := splitCommand("RUN ", aptCmd)
	dockerfileLines = append(dockerfileLines, strings.Join(aptLines, "\n"))

	// Формирование RUN блока для пользовательских команд, если они заданы.
	if len(c.Commands) > 0 {
		cmdCombined := strings.Join(c.Commands, " && ")
		cmdLines := splitCommand("RUN ", cmdCombined)
		dockerfileLines = append(dockerfileLines, strings.Join(cmdLines, "\n"))
	}

	dockerStr := strings.Join(dockerfileLines, "\n") + "\n"
	err := os.WriteFile(ContainerPath, []byte(dockerStr), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) CheckCommands() error {
	if len(c.Packages.Install) == 0 && len(c.Packages.Remove) == 0 && len(c.Commands) == 0 {
		return fmt.Errorf("конфигурационный файл локального образа не содержит изменений")
	}

	return nil
}

// Save записывает обратно в файл.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(lib.Env.PathImageFile, data, 0644)
}

// ConfigIsChange проверяем, изменился ли новый конфиг
func (c *Config) ConfigIsChange(ctx context.Context) (bool, error) {
	statusSame, err := IsLatestConfigSame(ctx, *c)
	if err != nil {
		return false, err
	}

	if statusSame {
		return false, nil
	}

	return true, nil
}

// SaveToDB сохраняем в историю
func (c *Config) SaveToDB(ctx context.Context) error {
	history := ImageHistory{
		ImageName: c.Image,
		Config:    c,
		ImageDate: time.Now().Format(time.RFC3339),
	}

	statusSame, err := c.ConfigIsChange(ctx)
	if err != nil {
		return err
	}

	// если изменился - сохраняем в базу
	if statusSame {
		err = SaveImageToDB(ctx, history)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddCommand добавляет команду в список Commands и сохраняет изменения в файл.
func (c *Config) AddCommand(cmd string) error {
	if contains(c.Commands, cmd) {
		return nil
	}

	c.Commands = append(c.Commands, cmd)
	return c.Save()
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (c *Config) AddInstallPackage(pkg string) error {
	if contains(c.Packages.Install, pkg) {
		return nil
	}

	if contains(c.Packages.Remove, pkg) {
		c.Packages.Remove = removeElement(c.Packages.Remove, pkg)
	}

	c.Packages.Install = append(c.Packages.Install, pkg)
	return c.Save()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (c *Config) AddRemovePackage(pkg string) error {
	if contains(c.Packages.Remove, pkg) {
		return nil
	}

	if contains(c.Packages.Install, pkg) {
		c.Packages.Install = removeElement(c.Packages.Install, pkg)
	}

	c.Packages.Remove = append(c.Packages.Remove, pkg)
	return c.Save()
}

// removeElement удаляет элемент из среза строк.
func removeElement(slice []string, element string) []string {
	var newSlice []string
	for _, v := range slice {
		if v != element {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}

// contains проверяет, содержит ли срез slice значение s.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// uniqueStrings возвращает новый срез, содержащий только уникальные элементы исходного среза.
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func generateFile(path string) (Config, error) {
	var cfg Config
	hostImage, err := GetHostImage()
	if err != nil {
		return Config{}, err
	}

	cfg.Image = hostImage.Status.Booted.Image.Image.Image
	cfg.Packages.Install = []string{}
	cfg.Packages.Remove = []string{}
	cfg.Commands = []string{}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return Config{}, err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// splitCommand разбивает команду на строки длиной не более 80 символов с отступом.
func splitCommand(prefix, cmd string) []string {
	const maxLineLength = 80
	words := strings.Fields(cmd)
	var lines []string
	currentLine := prefix
	for _, word := range words {
		// Если добавление следующего слова превышает максимальную длину строки.
		if len(currentLine)+len(word)+1 > maxLineLength {
			lines = append(lines, currentLine+" \\")
			currentLine = "    " + word // отступ для продолжения команды
		} else {
			if currentLine == prefix {
				currentLine += word
			} else {
				currentLine += " " + word
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}
