package common_types

import (
	"apm/internal/common/version"
)

type ExprData struct {
	Env     map[string]string
	Version version.Version
}