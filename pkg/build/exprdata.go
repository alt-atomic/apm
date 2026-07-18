package build

import (
	"fmt"
	"runtime"

	"altlinux.space/alt-atomic/apm/pkg/osutils"
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
	Version Version
	Arch    string
	GoArch  string
}

// newExprData builds the base expr data (env, version, arch); Modules is set per module.
func newExprData(version Version) ExprData {
	return ExprData{
		Env:     osutils.GetEnvMap(),
		Version: version,
		Arch:    detectArch(),
		GoArch:  runtime.GOARCH,
	}
}
