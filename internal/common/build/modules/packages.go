package modules

import (
	"context"
	"fmt"
	"strings"

	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

type PackagesBody struct {
	// Пакеты к установке
	Install []string `yaml:"install,omitempty" json:"install,omitempty"`

	// Пакеты к удалению
	Remove []string `yaml:"remove,omitempty" json:"remove,omitempty"`

	// Обновить ли базу данных до транзакции
	Update bool `yaml:"update,omitempty" json:"update,omitempty"`

	// Обновить ли пакеты до транзакции
	Upgrade bool `yaml:"upgrade,omitempty" json:"upgrade,omitempty"`

	// Удалить пакеты с зависимостями
	Depends bool `yaml:"depends,omitempty" json:"depends,omitempty"`

	// Переопределения конфигурации APT (например Dir::Cache::Archives=/tmp)
	Options map[string]string `yaml:"options,omitempty" json:"options,omitempty"`
}

func (b *PackagesBody) Execute(ctx context.Context, bc pkgbuild.RuntimeContext) (any, error) {
	return withDomain(ctx, bc, b.run)
}

func (b *PackagesBody) run(ctx context.Context, svc DomainContext) (any, error) {
	if len(b.Options) != 0 {
		svc.SetAptConfigOverrides(b.Options)
		defer svc.SetAptConfigOverrides(nil)
	}

	if b.Update {
		app.Log.Info("Updating package cache")
		if err := svc.UpdatePackages(ctx); err != nil {
			return nil, err
		}
	}
	if b.Upgrade {
		app.Log.Info("Upgrading packages")
		if err := svc.UpgradePackages(ctx); err != nil {
			return nil, err
		}
	}

	if len(b.Install) == 0 && len(b.Remove) == 0 {
		return nil, nil
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

	return nil, svc.CombineInstallRemovePackages(ctx, ops, false, b.Depends, false)
}

func init() { pkgbuild.Register(TypePackages, func() pkgbuild.Body { return &PackagesBody{} }) }
