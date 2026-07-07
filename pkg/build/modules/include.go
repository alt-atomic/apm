package modules

import (
	"context"

	"altlinux.space/alt-atomic/apm/pkg/build"
)

type IncludeBody struct {
	// yml конфиги для выполнения
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`
}

// IncludeTargets сообщает цели include движку для рекурсивной валидации.
func (b *IncludeBody) IncludeTargets() []string {
	return b.Targets
}

func (b *IncludeBody) Execute(ctx context.Context, svc build.RuntimeContext) (any, error) {
	var includeOutput = map[string]map[string]*build.MapModule{}

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

func init() { build.Register(TypeInclude, func() build.Body { return &IncludeBody{} }) }
