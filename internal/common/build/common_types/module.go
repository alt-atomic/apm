package common_types

import (
	"apm/internal/common/version"
	"fmt"
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
	} else {
		return fmt.Sprintf("id=%s", m.Id)
	}
}

type ExprData struct {
	Modules map[string]*MapModule
	Env     map[string]string
	Version version.Version
}
