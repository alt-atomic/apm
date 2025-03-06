package converter

import (
	"gopkg.in/yaml.v3"
	"os"
)

// Config описывает структуру конфигурационного файла.
type Config struct {
	Image    string `yaml:"image" json:"image"`
	Packages struct {
		Install []string `yaml:"install" json:"install"`
		Remove  []string `yaml:"remove" json:"remove"`
	} `yaml:"packages" json:"packages"`
	Commands []string `yaml:"commands" json:"commands"`
	FilePath string   `yaml:"-" json:"-"`
}

func generateFile(path string) (*Config, error) {
	var cfg Config
	cfg.Image = ""
	cfg.Packages.Install = []string{}
	cfg.Packages.Remove = []string{}
	cfg.Commands = []string{}
	cfg.FilePath = path

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ParseConfig читает YAML-конфигурацию из указанного файла,
func ParseConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return generateFile(path)
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.FilePath = path
	return &cfg, nil
}

// ParseConfig читает и парсит YAML-конфигурацию из указанного файла,
func (c *Config) getCommandFomFile() ([]string, error) {
	var commands []string

	commands = append(commands, "apt-get update")

	aptCmd := "apt-get -y"
	for _, pkg := range c.Packages.Install {
		aptCmd += " " + pkg + "+"
	}
	for _, pkg := range c.Packages.Remove {
		aptCmd += " " + pkg + "-"
	}
	commands = append(commands, aptCmd)
	commands = append(commands, c.Commands...)

	return commands, nil
}

// Save записывает обратно в файл.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.FilePath, data, 0644)
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
	newSlice := []string{}
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
