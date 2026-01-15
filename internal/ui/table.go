package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"poxy/pkg/manager"
)

// Table wraps tabwriter for consistent styling.
type Table struct {
	writer  *tabwriter.Writer
	headers []string
}

// NewTable creates a new table with default styling.
func NewTable(header []string) *Table {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	return &Table{
		writer:  w,
		headers: header,
	}
}

// NewTableWriter creates a new table that writes to a specific writer.
func NewTableWriter(w io.Writer, header []string) *Table {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	return &Table{
		writer:  tw,
		headers: header,
	}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(row []string) {
	fmt.Fprintln(t.writer, strings.Join(row, "\t"))
}

// Render outputs the table.
func (t *Table) Render() {
	// Print headers first in bold
	if len(t.headers) > 0 {
		headerRow := make([]string, len(t.headers))
		for i, h := range t.headers {
			headerRow[i] = Bold(strings.ToUpper(h))
		}
		fmt.Fprintln(t.writer, strings.Join(headerRow, "\t"))
	}
	t.writer.Flush()
}

// RenderWithHeaders outputs the table with headers already printed.
func (t *Table) RenderWithHeaders() {
	// Print headers first
	if len(t.headers) > 0 {
		headerRow := make([]string, len(t.headers))
		for i, h := range t.headers {
			headerRow[i] = Bold(strings.ToUpper(h))
		}
		fmt.Fprintln(t.writer, strings.Join(headerRow, "\t"))
	}
	t.writer.Flush()
}

// PrintPackages prints a list of packages in a formatted table.
func PrintPackages(packages []manager.Package) {
	if len(packages) == 0 {
		MutedMsg("No packages found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w, Bold("SOURCE")+"\t"+Bold("NAME")+"\t"+Bold("VERSION")+"\t"+Bold("DESCRIPTION"))

	for _, pkg := range packages {
		source := PackageSource.Sprint("[" + pkg.Source + "]")
		name := PackageName.Sprint(pkg.Name)
		version := PackageVersion.Sprint(pkg.Version)

		// Truncate description if too long
		desc := pkg.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		// Show installed indicator
		if pkg.Installed {
			name = name + " " + Installed.Sprint("[installed]")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", source, name, version, desc)
	}

	w.Flush()
}

// PrintPackageInfo prints detailed package information.
func PrintPackageInfo(info *manager.PackageInfo) {
	if info == nil {
		ErrorMsg("No package information available")
		return
	}

	HeaderMsg("Package Information")

	printField("Name", info.Name)
	printField("Version", info.Version)
	printField("Source", info.Source)

	if info.Description != "" {
		printField("Description", info.Description)
	}

	if info.Repository != "" {
		printField("Repository", info.Repository)
	}

	if info.License != "" {
		printField("License", info.License)
	}

	if info.URL != "" {
		printField("URL", info.URL)
	}

	if info.Maintainer != "" {
		printField("Maintainer", info.Maintainer)
	}

	if info.Size != "" {
		printField("Size", info.Size)
	}

	if len(info.Dependencies) > 0 {
		printField("Dependencies", strings.Join(info.Dependencies, ", "))
	}

	if !info.InstallDate.IsZero() {
		printField("Installed", info.InstallDate.Format("2006-01-02 15:04:05"))
	}
}

// printField prints a single field with formatting.
func printField(label, value string) {
	fmt.Printf("  %s: %s\n", Cyan(label), value)
}

// PrintSearchResults prints search results grouped by source.
func PrintSearchResults(packages []manager.Package) {
	if len(packages) == 0 {
		MutedMsg("No packages found")
		return
	}

	// Group by source
	grouped := make(map[string][]manager.Package)
	for _, pkg := range packages {
		grouped[pkg.Source] = append(grouped[pkg.Source], pkg)
	}

	totalCount := len(packages)
	sourceCount := len(grouped)

	HeaderMsg("Found %d results across %d sources", totalCount, sourceCount)

	for source, pkgs := range grouped {
		fmt.Printf("\n%s (%d):\n", PackageSource.Sprint("["+source+"]"), len(pkgs))

		for _, pkg := range pkgs {
			name := PackageName.Sprint(pkg.Name)
			version := ""
			if pkg.Version != "" {
				version = " " + PackageVersion.Sprint(pkg.Version)
			}

			installedMark := ""
			if pkg.Installed {
				installedMark = " " + Installed.Sprint("[installed]")
			}

			fmt.Printf("  %s%s%s\n", name, version, installedMark)

			if pkg.Description != "" {
				desc := pkg.Description
				if len(desc) > 70 {
					desc = desc[:67] + "..."
				}
				MutedMsg("    %s", desc)
			}
		}
	}
}

// PrintSystemInfo prints system information.
func PrintSystemInfo(osName, arch, distro, prettyName, nativeManager string, availableManagers []string) {
	HeaderMsg("System Information")

	printField("Operating System", prettyName)
	printField("Architecture", arch)

	if distro != "" {
		printField("Distribution", distro)
	}

	if nativeManager != "" {
		printField("Native Package Manager", nativeManager)
	}

	if len(availableManagers) > 0 {
		printField("Available Managers", strings.Join(availableManagers, ", "))
	}
}
