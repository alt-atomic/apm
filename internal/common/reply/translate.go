package reply

import (
	"apm/internal/common/app"
)

// TranslateKey принимает ключ и возвращает английский текст.
func TranslateKey(key string) string {
	switch key {
	case "aliases":
		return app.T_("Aliases")
	case "desktopPaths":
		return app.T_("Desktop Paths")
	case "consolePaths":
		return app.T_("Console Paths")
	case "architecture":
		return app.T_("Architecture")
	case "result":
		return app.T_("Result")
	case "appStream":
		return app.T_("Application Information")
	case "downloadSize":
		return app.T_("Downloaded Size")
	case "installSize":
		return app.T_("Installed Size")
	case "package":
		return app.T_("Package")
	case "isApp":
		return app.T_("This Application")
	case "typePackage":
		return app.T_("Type Package")
	case "count":
		return app.T_("Count")
	case "isConsole":
		return app.T_("Console Application")
	case "packageInfo":
		return app.T_("Package Information")
	case "install":
		return app.T_("Install")
	case "store":
		return app.T_("Storage Type")
	case "timestamp":
		return app.T_("Date")
	case "imageDigest":
		return app.T_("Image Digest")
	case "os":
		return app.T_("Distribution")
	case "container":
		return app.T_("Container")
	case "name":
		return app.T_("Name")
	case "extraInstalled":
		return app.T_("Extra Installed")
	case "upgradedCount":
		return app.T_("Upgraded Count")
	case "bootedImage":
		return app.T_("Booted Image")
	case "removedPackages":
		return app.T_("Removed Packages")
	case "provides":
		return app.T_("Provides")
	case "providers":
		return app.T_("Providers")
	case "version":
		return app.T_("Version")
	case "history":
		return app.T_("History")
	case "depends":
		return app.T_("Dependencies")
	case "installedSize":
		return app.T_("Installed Size")
	case "removedCount":
		return app.T_("Removed Count")
	case "upgradedPackages":
		return app.T_("Upgraded Packages")
	case "packageName":
		return app.T_("Package Name")
	case "image":
		return app.T_("Image")
	case "commands":
		return app.T_("Commands")
	case "maintainer":
		return app.T_("Maintainer")
	case "versionInstalled":
		return app.T_("Installed Version")
	case "remove":
		return app.T_("Remove")
	case "containers":
		return app.T_("Containers")
	case "paths":
		return app.T_("Paths")
	case "description":
		return app.T_("Description")
	case "date":
		return app.T_("Date")
	case "newInstalledCount":
		return app.T_("Newly Installed Count")
	case "active":
		return app.T_("Active")
	case "info":
		return app.T_("Information")
	case "totalCount":
		return app.T_("Total Count")
	case "installed":
		return app.T_("Installed")
	case "manager":
		return app.T_("Package Manager")
	case "lastChangelog":
		return app.T_("Last Changelog")
	case "section":
		return app.T_("Section")
	case "spec":
		return app.T_("Specification")
	case "booted":
		return app.T_("Booted")
	case "staged":
		return app.T_("Staged")
	case "size":
		return app.T_("Size")
	case "newInstalledPackages":
		return app.T_("Newly Installed Packages")
	case "notUpgradedCount":
		return app.T_("Not Upgraded Count")
	case "containerName":
		return app.T_("Container Name")
	case "config":
		return app.T_("Configuration")
	case "exporting":
		return app.T_("Exporting")
	case "status":
		return app.T_("Status")
	case "imageDate":
		return app.T_("Image Date")
	case "packages":
		return app.T_("Packages")
	case "filename":
		return app.T_("Filename")
	case "containerInfo":
		return app.T_("Container Information")
	case "imageName":
		return app.T_("Image Name")
	case "transport":
		return app.T_("Transport")
	case "pinned":
		return app.T_("Pinned")
	case "list":
		return app.T_("List")
	case "kernel":
		return app.T_("Kernel")
	case "kernels":
		return app.T_("Kernels")
	case "currentKernel":
		return app.T_("Current Kernel")
	case "latestKernel":
		return app.T_("Latest Kernel")
	case "preview":
		return app.T_("Preview")
	case "ageInDays":
		return app.T_("Age in Days")
	case "buildTime":
		return app.T_("Build Time")
	case "flavour":
		return app.T_("Flavour")
	case "fullVersion":
		return app.T_("Full Version")
	case "isInstalled":
		return app.T_("Is Installed")
	case "isRunning":
		return app.T_("Is Running")
	case "release":
		return app.T_("Release")
	case "modules":
		return app.T_("Modules")
	case "kept":
		return app.T_("Kept")
	case "reasons":
		return app.T_("Reasons")
	case "versionRaw":
		return app.T_("Version Raw")
	case "keptKernels":
		return app.T_("Kept Kernels")
	case "removeKernels":
		return app.T_("Remove kernels")
	case "InstalledModules":
		return app.T_("Installed Modules")
	case "selectedModules":
		return app.T_("Selected Modules")
	case "missingModules":
		return app.T_("Missing Modules")
	case "updateAvailable":
		return app.T_("Available Update")
	default:
		return app.T_(key)
	}
}
