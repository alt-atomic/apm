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
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

var (
	goodInitrdMethods = []string{
		"auto",
		"none",
		"dracut",
		"make-initrd",
	}
	defaultDracutPath      = "/usr/bin/dracut"
	defaultMakeInitrdPath  = "/usr/sbin/make-initrd"
	kernelDir              = "/usr/lib/modules"
	bootVmlinuzTemplate    = "/boot/vmlinuz-%s"
	plymouthThemesDir      = "/usr/share/plymouth/themes"
	plymouthConfigFile     = "/etc/plymouth/plymouthd.conf"
	plymouthKargsPath      = "/usr/lib/bootc/kargs.d/00-plymouth.toml"
	plymouthDracutConfPath = "/usr/lib/dracut/dracut.conf.d/00-plymouth.conf"
)

type KernelInfo struct {
	// Версия ядра
	Flavour string `yaml:"flavour,omitempty" json:"flavour,omitempty"`

	// Модуля ядра
	Modules []string `yaml:"modules,omitempty" json:"modules,omitempty"`

	// Включать хедеры
	IncludeHeaders bool `yaml:"include-headers,omitempty" json:"include-headers,omitempty"`
}

func (i *KernelInfo) IsEmpty() bool {
	return i.Flavour == "" && len(i.Modules) == 0 && !i.IncludeHeaders
}

type Initrd struct {
	// Поддерживаются: dracut, auto. Если пусто и прописан один из
	// flavour, modules, inckude-headers, то используется auto
	Method string `yaml:"method,omitempty" json:"method,omitempty"`

	// Тема плимут
	PlymouthTheme string `yaml:"plymouth-theme,omitempty" json:"plymouth-theme,omitempty"`
}

func (i *Initrd) IsEmpty() bool {
	return i.Method == "" && i.PlymouthTheme == ""
}

type KernelBody struct {
	KernelInfo KernelInfo `yaml:"kernel-info,omitempty" json:"kernel-info"`

	Initrd Initrd `yaml:"initrd,omitempty" json:"initrd"`
}

func (b *KernelBody) Validate() error {
	if !b.Initrd.IsEmpty() {
		if !slices.Contains(goodInitrdMethods, b.Initrd.Method) {
			return fmt.Errorf(app.T_("unknown initrd method %s"), b.Initrd.Method)
		}
	}

	return nil
}

func (b *KernelBody) Execute(ctx context.Context, svc Service) (any, error) {
	b.Validate()

	var shouldInstallKernel = !b.KernelInfo.IsEmpty()
	var shouldRebuildInitrd = !b.Initrd.IsEmpty() || shouldInstallKernel

	app.Log.Warn(shouldInstallKernel)
	app.Log.Warn(shouldRebuildInitrd)

	if shouldInstallKernel {
		mgr := svc.KernelManager()
		modules := append([]string{}, b.KernelInfo.Modules...)

		var latestKernelInfo *service.Info
		var err error
		if b.KernelInfo.Flavour != "" {
			latestKernelInfo, err = mgr.FindLatestKernel(ctx, b.KernelInfo.Flavour)
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

		additionalPackages, _ := mgr.AutoSelectHeadersAndFirmware(ctx, toInstall, b.KernelInfo.IncludeHeaders)
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
		if err = mgr.InstallKernel(ctx, toInstall, modules, b.KernelInfo.IncludeHeaders, false); err != nil {
			return nil, err
		}

		// TODO: Заменить на более точечное обновление, как в kernel service
		app.Log.Info("Updating packages DB for kernel")
		if err = svc.UpdatePackages(ctx); err != nil {
			return nil, err
		}

		if svc.IsAtomic() {
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
		}
	}

	if shouldRebuildInitrd {
		if b.Initrd.PlymouthTheme != "" {
			plymouthPaths := []string{plymouthKargsPath, plymouthDracutConfPath}

			if b.Initrd.PlymouthTheme == "disabled" {
				if _, err := os.Stat(plymouthConfigFile); err == nil {
					if err := os.WriteFile(plymouthConfigFile, []byte(""), 0644); err != nil {
						return nil, err
					}
					for _, p := range plymouthPaths {
						if err := os.RemoveAll(p); err != nil {
							return nil, err
						}
					}
				}
			} else {
				var themes []string
				if _, err := os.Stat(plymouthThemesDir); err == nil {
					files, err := os.ReadDir(plymouthThemesDir)
					if err != nil {
						return nil, err
					}
					for _, file := range files {
						themes = append(themes, file.Name())
					}
				}

				if !slices.Contains(themes, b.Initrd.PlymouthTheme) {
					filters := map[string]any{
						"name": fmt.Sprintf("plymouth-theme-%s", b.Initrd.PlymouthTheme),
					}
					packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
					if err != nil {
						return nil, err
					}
					if len(packages) == 0 {
						return nil, fmt.Errorf("no plymouth theme found for %s", b.Initrd.PlymouthTheme)
					}

					var pkgsNames []string
					for _, pkg := range packages {
						pkgsNames = append(pkgsNames, pkg.Name)
					}

					packagesBody := &PackagesBody{Install: pkgsNames}

					if _, err = packagesBody.Execute(ctx, svc); err != nil {
						return nil, err
					}
				}

				plymouthConfig := strings.Join([]string{
					"[Daemon]",
					fmt.Sprintf("Theme=%s", b.Initrd.PlymouthTheme),
					"ShowDelay=0",
					"DeviceTimeout=10",
				}, "\n") + "\n"

				if err := os.MkdirAll(path.Dir(plymouthConfigFile), 0644); err != nil {
					return nil, err
				}
				if err := os.WriteFile(plymouthConfigFile, []byte(plymouthConfig), 0644); err != nil {
					return nil, err
				}

				for _, p := range plymouthPaths {
					if err := os.MkdirAll(path.Dir(p), 0644); err != nil {
						return nil, err
					}
				}

				if svc.IsAtomic() {
					if err := os.WriteFile(plymouthKargsPath, []byte(`kargs = ["rhgb", "quiet", "splash", "plymouth.enable=1", "rd.plymouth=1"]`+"\n"), 0644); err != nil {
						return nil, err
					}
					if err := os.WriteFile(plymouthDracutConfPath, []byte(`add_dracutmodules+=" plymouth "`+"\n"), 0644); err != nil {
						return nil, err
					}
				}
			}
		}

		switch b.Initrd.Method {
		case "dracut":
			err := rebuildDracut(ctx, defaultDracutPath)
			if err != nil {
				return nil, err
			}
		case "make-initrd":
			err := rebuildMakeInitrd(ctx, defaultMakeInitrdPath)
			if err != nil {
				return nil, err
			}
		case "none":
			return nil, nil
		default:
			dracutPath, dracutErr := exec.LookPath(defaultDracutPath)
			makeInitrdPath, makeInitrdErr := exec.LookPath(defaultMakeInitrdPath)
			if dracutPath != "" && dracutErr == nil {
				if err := rebuildDracut(ctx, dracutPath); err != nil {
					return nil, err
				}
			} else if makeInitrdPath != "" && makeInitrdErr == nil {
				if err := rebuildMakeInitrd(ctx, makeInitrdPath); err != nil {
					return nil, err
				}
			}
		}
	}

	return nil, nil
}

func rebuildMakeInitrd(_ context.Context, _ string) error {
	app.Log.Error("make-initd not supported")

	return nil
}

func rebuildDracut(ctx context.Context, dracutExecutable string) error {
	app.Log.Info("Rebuild initramfs via dracut")

	kernelVersion, err := LatestInstalledKernelVersion()
	if err != nil {
		return err
	}

	if err = osutils.ExecSh(ctx, fmt.Sprintf("depmod -a -v '%s'", kernelVersion), "", true); err != nil {
		return err
	}

	return osutils.ExecSh(ctx, fmt.Sprintf("%s --force '%s/%s/initramfs.img' %s", dracutExecutable, kernelDir, kernelVersion, kernelVersion), "", false)
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
