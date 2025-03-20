package reply

// TranslateKey принимает ключ и возвращает английский текст.
func TranslateKey(key string) string {
	switch key {
	case "package":
		return "Package"
	case "count":
		return "Count"
	case "isConsole":
		return "Console Application"
	case "packageInfo":
		return "Package Information"
	case "install":
		return "Install"
	case "store":
		return "Storage Type"
	case "timestamp":
		return "Date"
	case "imageDigest":
		return "Image Digest"
	case "os":
		return "Distribution"
	case "container":
		return "Container"
	case "name":
		return "Name"
	case "extraInstalled":
		return "Extra Installed"
	case "upgradedCount":
		return "Upgraded Count"
	case "bootedImage":
		return "Booted Image"
	case "removedPackages":
		return "Removed Packages"
	case "providers":
		return "Providers"
	case "version":
		return "Version"
	case "history":
		return "History"
	case "depends":
		return "Dependencies"
	case "installedSize":
		return "Installed Size"
	case "removedCount":
		return "Removed Count"
	case "upgradedPackages":
		return "Upgraded Packages"
	case "packageName":
		return "Package Name"
	case "image":
		return "Image"
	case "commands":
		return "Commands"
	case "maintainer":
		return "Maintainer"
	case "versionInstalled":
		return "Installed Version"
	case "remove":
		return "Remove"
	case "containers":
		return "Containers"
	case "paths":
		return "Paths"
	case "description":
		return "Description"
	case "date":
		return "Date"
	case "newInstalledCount":
		return "Newly Installed Count"
	case "active":
		return "Active"
	case "info":
		return "Information"
	case "totalCount":
		return "Total Count"
	case "installed":
		return "Installed"
	case "manager":
		return "Package Manager"
	case "lastChangelog":
		return "Last Changelog"
	case "section":
		return "Section"
	case "spec":
		return "Specification"
	case "booted":
		return "Booted"
	case "staged":
		return "Staged"
	case "size":
		return "Size"
	case "newInstalledPackages":
		return "Newly Installed Packages"
	case "notUpgradedCount":
		return "Not Upgraded Count"
	case "containerName":
		return "Container Name"
	case "config":
		return "Configuration"
	case "exporting":
		return "Exporting"
	case "status":
		return "Status"
	case "imageDate":
		return "Image Date"
	case "packages":
		return "Packages"
	case "filename":
		return "Filename"
	case "containerInfo":
		return "Container Information"
	case "imageName":
		return "Image Name"
	case "transport":
		return "Transport"
	case "pinned":
		return "Pinned"
	case "list":
		return "List"
	default:
		return key
	}
}
