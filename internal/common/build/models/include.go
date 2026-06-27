package models

import (
	"apm/internal/common/build/common_types"
	"apm/internal/common/osutils"
	"context"
	"crypto/sha256"
)

type IncludeBody struct {
	// YAML конфиги для выполнения
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

func (b *IncludeBody) CacheHash(_ string) (string, error) {
	h := sha256.New()
	for _, target := range b.Targets {
		if !osutils.IsURL(target) {
			continue
		}
		data, err := fetchURL(target)
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	return HashBody(b, h.Sum(nil))
}
