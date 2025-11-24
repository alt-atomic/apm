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
		if err := svc.ExecuteInclude(ctx, target); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
