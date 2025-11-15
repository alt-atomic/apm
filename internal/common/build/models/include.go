package models

import (
	"apm/internal/common/build/core"
	"context"
)

type IncludeBody struct {
	// yml конфиги для выполнения
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty"`
}

func (b *IncludeBody) Check() error {
	return nil
}

func (b *IncludeBody) Execute(ctx context.Context, cfgService core.Service) error {
	for _, target := range b.Targets {
		modules, err := core.ReadAndParseModulesYaml(target)
		if err != nil {
			return err
		}
		for _, module := range *modules {
			if err = cfgService.ExecuteModule(ctx, module); err != nil {
				return err
			}
		}
	}
	return nil
}
