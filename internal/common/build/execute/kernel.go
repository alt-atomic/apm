package execute

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/build/core"
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

func Kernel(ctx context.Context, svc Service) error {
	kernelCfg := svc.Config().Kernel
	if kernelCfg.Flavour == "" && len(kernelCfg.Modules) == 0 && !kernelCfg.IncludeHeaders {
		return nil
	}

	mgr := svc.KernelManager()
	modules := append([]string{}, kernelCfg.Modules...)

	var latestKernelInfo *service.Info
	var err error
	if kernelCfg.Flavour != "" {
		latestKernelInfo, err = mgr.FindLatestKernel(ctx, kernelCfg.Flavour)
		if err != nil {
			return err
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
		return errors.New("kernel must be specified")
	}

	additionalPackages, _ := mgr.AutoSelectHeadersAndFirmware(ctx, toInstall, kernelCfg.IncludeHeaders)
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
			return err
		}

		entries, err := os.ReadDir(core.KernelDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = os.RemoveAll(filepath.Join(core.KernelDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	app.Log.Info(fmt.Sprintf("Installing kernel %s with modules: %s", toInstall.Flavour, strings.Join(modules, ", ")))
	if err = mgr.InstallKernel(ctx, toInstall, modules, kernelCfg.IncludeHeaders, false); err != nil {
		return err
	}

	app.Log.Info("Updating packages DB")
	if err = svc.UpdatePackages(ctx); err != nil {
		return err
	}

	latestInstalledKernelVersion, err := LatestInstalledKernelVersion()
	if err != nil {
		return err
	}

	app.Log.Info("Copy vmlinuz")
	return osutils.Copy(
		fmt.Sprintf(core.BootVmlinuzTemplate, latestInstalledKernelVersion),
		fmt.Sprintf("%s/%s/vmlinuz", core.KernelDir, latestInstalledKernelVersion),
		true,
	)
}

func LatestInstalledKernelVersion() (string, error) {
	files, err := os.ReadDir(core.KernelDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no kernel versions found in %s", core.KernelDir)
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
