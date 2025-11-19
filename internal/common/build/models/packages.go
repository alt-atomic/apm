package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"strings"
)

type PackagesBody struct {
	// Пакеты к установке
	Install []string `yaml:"install,omitempty" json:"install,omitempty"`

	// Пакеты к удалению
	Remove []string `yaml:"remove,omitempty" json:"remove,omitempty"`

	// Обновить ли базу данных до транзакции
	Update bool `yaml:"update,omitempty" json:"update,omitempty"`

	// Обновить ли пакеты до тарнзакции
	Upgrade bool `yaml:"upgrade,omitempty" json:"upgrade,omitempty"`
}

func (b *PackagesBody) Execute(ctx context.Context, svc Service) error {
	if b.Update {
		app.Log.Info("Updating package cache")
		if err := svc.UpdatePackages(ctx); err != nil {
			return err
		}
	}
	if b.Upgrade {
		app.Log.Info("Upgrading packages")
		if err := svc.UpdatePackages(ctx); err != nil {
			return err
		}
	}

	if len(b.Install) == 0 && len(b.Remove) == 0 {
		return nil
	}

	var text []string
	if len(b.Install) != 0 {
		text = append(text, fmt.Sprintf("installing %s", strings.Join(b.Install, ", ")))
	}
	if len(b.Remove) != 0 {
		text = append(text, fmt.Sprintf("removing %s", strings.Join(b.Remove, ", ")))
	}
	if len(text) != 0 {
		app.Log.Info(osutils.Capitalize(strings.Join(text, " and ")))
	}

	var ops []string
	for _, p := range b.Install {
		ops = append(ops, p+"+")
	}
	for _, p := range b.Remove {
		ops = append(ops, p+"-")
	}

	return svc.CombineInstallRemovePackages(ctx, ops, false, false)
}
