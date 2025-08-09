package examples

import (
	"apm/internal/common/binding/apt"
	"apm/internal/common/binding/apt/lib"
	"fmt"
	"log"
)

func Test(searchPattern string) {
	aptService := apt.NewActions()

	packages, err := aptService.Search(searchPattern)
	if err != nil {
		log.Fatalf("Failed to search packages: %v", err)
	}
	defer aptService.Close()

	fmt.Printf("\nFound %d packages:\n", len(packages))
	for _, pkg := range packages {
		fmt.Printf("Package: %s (%s)\n", pkg.Name, pkg.Version)
		if pkg.ShortDescription != "" {
			fmt.Printf("  Description: %s\n", pkg.ShortDescription)
		}
		fmt.Printf("  Maintainer: %s\n", pkg.Maintainer)
		fmt.Printf("  Section: %s\n", pkg.Section)
		fmt.Printf("  Architecture: %s\n", pkg.Architecture)
		fmt.Printf("  PackageID: %s\n", pkg.PackageID)
		fmt.Printf("  Recommends: %s\n", pkg.Recommends)
		fmt.Printf("  Filename: %s\n", pkg.Filename)
		fmt.Printf("  Description: %s\n", pkg.Description)
		fmt.Printf("  State: ")
		switch pkg.State {
		case lib.PackageStateNotInstalled:
			fmt.Println("Not installed")
		case lib.PackageStateInstalled:
			fmt.Println("Installed")
		case lib.PackageStateConfigFiles:
			fmt.Println("Config files only")
		default:
			fmt.Printf("State %d\n", int(pkg.State))
		}

		fmt.Println()
	}

	if len(packages) == 0 {
		fmt.Printf("No packages found matching '%s'\n", searchPattern)
	}
}
