package models

import (
	"context"
)

type IncludeBody struct {
	// yml конфиги для выполнения
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`
}

func (b *IncludeBody) Execute(ctx context.Context, svc Service) (any, error) {
	for _, target := range b.Targets {
		err := svc.ExecuteInclude(ctx, target)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (b *IncludeBody) Hash(_ string, env map[string]string) string {
	return hashWithEnv(b, env)
}
