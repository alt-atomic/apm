package models

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/osutils"
	"apm/internal/common/reply"
	"apm/internal/kernel/service"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	goodInitrdMethods = []string{
		"dracut",
		// "make-initrd",
	}
	kernelDir           = "/usr/lib/modules"
	bootVmlinuzTemplate = "/boot/vmlinuz-%s"
)

type KernelBody struct {
	// Версия ядра
	Flavour string `yaml:"flavour,omitempty" json:"flavour,omitempty"`

	// Модуля ядра
	Modules []string `yaml:"modules,omitempty" json:"modules,omitempty"`

	// Включать хедеры
	IncludeHeaders bool `yaml:"include-headers,omitempty" json:"include-headers,omitempty"`

	// Пересобрать initramfs
	RebuildInitrdMethod string `yaml:"rebuild-initrd-method,omitempty" json:"rebuild-initrd-method,omitempty"`
}

func (b *KernelBody) Check() error {
	return nil
}

func (b *KernelBody) Execute(ctx context.Context, svc Service) (any, error) {
	if b.RebuildInitrdMethod == "" || !slices.Contains(goodInitrdMethods, b.RebuildInitrdMethod) {
		return nil, fmt.Errorf(app.T_("unknown initrd method %s"), b.RebuildInitrdMethod)
	}

	mgr := svc.KernelManager()
	modules := append([]string{}, b.Modules...)

	var latestKernelInfo *service.Info
	var err error
	if b.Flavour != "" {
		latestKernelInfo, err = mgr.FindLatestKernel(ctx, b.Flavour)
		if err != nil {
			return nil, err
		}
	}

	currentKernel, _ := currentKernelInfo(ctx, svc)

	var toInstall *service.Info
	if latestKernelInfo != nil {
		toInstall = latestKernelInfo
	} else if currentKernel != nil {
		toInstall = currentKernel
		inheritedModules, _ := mgr.InheritModulesFromKernel(toInstall, currentKernel)
		if len(inheritedModules) > 0 {
			modules = append(modules, inheritedModules...)
		}
	} else {
		return nil, errors.New("kernel must be specified")
	}

	additionalPackages, _ := mgr.AutoSelectHeadersAndFirmware(ctx, toInstall, b.IncludeHeaders)
	for _, pkg := range additionalPackages {
		if strings.HasPrefix(pkg, "kernel-modules-") && strings.HasSuffix(pkg, fmt.Sprintf("-%s", toInstall.Flavour)) {
			moduleName := strings.TrimPrefix(pkg, "kernel-modules-")
			moduleName = strings.TrimSuffix(moduleName, fmt.Sprintf("-%s", toInstall.Flavour))
			if !slices.Contains(modules, moduleName) {
				modules = append(modules, moduleName)
			}
		}
	}

	if currentKernel != nil {
		app.Log.Info(fmt.Sprintf("Removing current kernel %s", currentKernel.Flavour))
		if err = mgr.RemoveKernel(currentKernel, true); err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(kernelDir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err = os.RemoveAll(filepath.Join(kernelDir, entry.Name())); err != nil {
				return nil, err
			}
		}
	}

	app.Log.Info(fmt.Sprintf("Installing kernel %s with modules: %s", toInstall.Flavour, strings.Join(modules, ", ")))
	if err = mgr.InstallKernel(ctx, toInstall, modules, b.IncludeHeaders, false); err != nil {
		return nil, err
	}

	app.Log.Info("Updating packages DB")
	if err = svc.UpdatePackages(ctx); err != nil {
		return nil, err
	}

	latestInstalledKernelVersion, err := LatestInstalledKernelVersion()
	if err != nil {
		return nil, err
	}

	app.Log.Info("Copy vmlinuz")
	err = osutils.Copy(
		fmt.Sprintf(bootVmlinuzTemplate, latestInstalledKernelVersion),
		fmt.Sprintf("%s/%s/vmlinuz", kernelDir, latestInstalledKernelVersion),
		true,
	)
	if err != nil {
		return nil, err
	}

	if b.RebuildInitrdMethod != "" {
		switch b.RebuildInitrdMethod {
		case "dracut":
			app.Log.Info("Rebuild initramfs via dracut")

			kernelVersion, err := LatestInstalledKernelVersion()
			if err != nil {
				return nil, err
			}

			if _, err = osutils.ExecShWithOutput(ctx, fmt.Sprintf("depmod -a -v '%s'", kernelVersion), "", true); err != nil {
				return nil, err
			}

			_, err = osutils.ExecShWithOutput(ctx, fmt.Sprintf("dracut --force '%s/%s/initramfs.img' %s", kernelDir, kernelVersion, kernelVersion), "", false)

			return nil, err
		default:
			panic("unknown initrd method")
		}
	}
	return nil, nil
}

func LatestInstalledKernelVersion() (string, error) {
	files, err := os.ReadDir(kernelDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no kernel versions found in %s", kernelDir)
	}

	var names []string
	for _, file := range files {
		names = append(names, file.Name())
	}
	slices.Sort(names)
	return names[0], nil
}

func currentKernelInfo(ctx context.Context, svc Service) (*service.Info, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("kernel.CurrentKernel"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("kernel.CurrentKernel"))

	filters := map[string]any{
		"typePackage": int(_package.PackageTypeSystem),
		"name":        "kernel-image-",
		"installed":   true,
	}
	packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, nil
	}

	kernelInfo := svc.KernelManager().ParseKernelPackageFromDB(packages[0])
	if kernelInfo == nil {
		return nil, errors.New(app.T_("failed to parse kernel package from database"))
	}

	kernelInfo.IsRunning = true
	kernelInfo.IsInstalled = true

	return kernelInfo, nil
}
