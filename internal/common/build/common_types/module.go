package common_types

import (
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/version"
)

type MapModule struct {
	Name   string
	Type   string
	Id     string
	If     bool
	Output map[string]string
}

func (m MapModule) GetLabel() any {
	if m.Name != "" {
		return m.Name
	}

	return fmt.Sprintf("id=%s", m.Id)
}

type ExprData struct {
	Modules map[string]*MapModule
	Env     map[string]string
	Version version.Version
}
