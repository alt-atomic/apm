package models

import (
	"apm/internal/common/build/common_types"
	"context"
)

type IncludeBody struct {
	// yml конфиги для выполнения
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`
}

func (b *IncludeBody) Execute(ctx context.Context, svc Service) (any, error) {
	var includeOutput = map[string]map[string]*common_types.MapModule{}

	for _, target := range b.Targets {
		if output, err := svc.ExecuteInclude(ctx, target); err != nil {
			return nil, err
		} else {
			if len(b.Targets) == 1 {
				return output, nil
			}
			includeOutput[target] = output
		}
	}
	return includeOutput, nil
}
