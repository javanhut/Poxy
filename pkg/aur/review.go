package aur

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// ReviewResult represents the user's decision after reviewing a PKGBUILD.
type ReviewResult int

const (
	// ReviewAccept means the user accepts and wants to continue
	ReviewAccept ReviewResult = iota
	// ReviewReject means the user rejects and wants to abort
	ReviewReject
	// ReviewViewFull means the user wants to view the full PKGBUILD
	ReviewViewFull
)

// Reviewer provides interactive PKGBUILD review functionality.
type Reviewer struct {
	reader    *bufio.Reader
	useColors bool
}

// NewReviewer creates a new PKGBUILD reviewer.
func NewReviewer() *Reviewer {
	return &Reviewer{
		reader:    bufio.NewReader(os.Stdin),
		useColors: true,
	}
}

// SetColors enables or disables colored output.
func (r *Reviewer) SetColors(enabled bool) {
	r.useColors = enabled
}

// Review displays a PKGBUILD review and prompts for user action.
// Returns true if the user accepts, false if they reject.
func (r *Reviewer) Review(pkg *Package, pkgbuild *PKGBUILD) bool {
	for {
		r.displayReview(pkg, pkgbuild)
		result := r.promptAction()

		switch result {
		case ReviewAccept:
			return true
		case ReviewReject:
			return false
		case ReviewViewFull:
			r.displayFullPKGBUILD(pkgbuild)
		}
	}
}

// displayReview shows the PKGBUILD review summary.
func (r *Reviewer) displayReview(pkg *Package, pkgbuild *PKGBUILD) {
	titleColor := color.New(color.FgCyan, color.Bold)
	labelColor := color.New(color.FgWhite, color.Bold)
	valueColor := color.New(color.FgWhite)
	warnColor := color.New(color.FgYellow, color.Bold)
	errorColor := color.New(color.FgRed, color.Bold)
	successColor := color.New(color.FgGreen)

	fmt.Println()
	titleColor.Printf("=== PKGBUILD Review: %s ===\n", pkgbuild.Name())
	fmt.Println()

	// Package info from AUR
	labelColor.Print("AUR Info:\n")
	valueColor.Printf("  Maintainer:  %s\n", pkg.Maintainer)
	if pkg.IsOrphan() {
		warnColor.Print("  WARNING: Package is orphaned (no maintainer)\n")
	}
	valueColor.Printf("  Votes:       %d\n", pkg.NumVotes)
	valueColor.Printf("  Popularity:  %.2f\n", pkg.Popularity)
	valueColor.Printf("  Last Update: %s\n", pkg.LastModifiedTime().Format("2006-01-02"))
	if pkg.IsOutOfDate() {
		warnColor.Printf("  WARNING: Package marked out-of-date since %s\n",
			pkg.OutOfDateTime().Format("2006-01-02"))
	}
	fmt.Println()

	// PKGBUILD info
	labelColor.Print("Package Info:\n")
	valueColor.Printf("  Name:        %s\n", pkgbuild.Name())
	valueColor.Printf("  Version:     %s\n", pkgbuild.FullVersion())
	valueColor.Printf("  Description: %s\n", pkgbuild.PkgDesc)
	if pkgbuild.URL != "" {
		valueColor.Printf("  URL:         %s\n", pkgbuild.URL)
	}
	fmt.Println()

	// Dependencies
	if len(pkgbuild.Depends) > 0 || len(pkgbuild.MakeDepends) > 0 {
		labelColor.Print("Dependencies:\n")
		if len(pkgbuild.Depends) > 0 {
			valueColor.Printf("  Runtime:     %s\n", strings.Join(pkgbuild.Depends, ", "))
		}
		if len(pkgbuild.MakeDepends) > 0 {
			valueColor.Printf("  Build:       %s\n", strings.Join(pkgbuild.MakeDepends, ", "))
		}
		fmt.Println()
	}

	// Sources
	if urls := pkgbuild.SourceURLs(); len(urls) > 0 {
		labelColor.Print("Source URLs:\n")
		for _, url := range urls {
			valueColor.Printf("  - %s\n", url)
		}
		fmt.Println()
	}

	// Build functions
	labelColor.Print("Build Functions:\n")
	funcs := []string{}
	if pkgbuild.HasPrepare {
		funcs = append(funcs, "prepare()")
	}
	if pkgbuild.HasBuild {
		funcs = append(funcs, "build()")
	}
	if pkgbuild.HasCheck {
		funcs = append(funcs, "check()")
	}
	if pkgbuild.HasPackage {
		funcs = append(funcs, "package()")
	}
	successColor.Printf("  %s\n", strings.Join(funcs, ", "))
	fmt.Println()

	// Security warnings
	if pkgbuild.HasDangerousCommands() {
		errorColor.Print("SECURITY WARNINGS:\n")
		for _, cmd := range pkgbuild.DangerousCommands {
			errorColor.Printf("  Line %d: %s\n", cmd.Line, cmd.Reason)
			valueColor.Printf("    %s\n", truncate(cmd.Command, 70))
		}
		fmt.Println()
	} else {
		successColor.Print("No obvious security issues detected.\n")
		fmt.Println()
	}
}

// displayFullPKGBUILD shows the full PKGBUILD content.
func (r *Reviewer) displayFullPKGBUILD(pkgbuild *PKGBUILD) {
	titleColor := color.New(color.FgCyan, color.Bold)
	lineColor := color.New(color.FgYellow)

	fmt.Println()
	titleColor.Print("=== Full PKGBUILD ===\n")
	fmt.Println()

	lines := strings.Split(pkgbuild.RawContent, "\n")
	for i, line := range lines {
		lineColor.Printf("%4d | ", i+1)
		fmt.Println(line)
	}

	fmt.Println()
	fmt.Print("Press Enter to continue...")
	_, _ = r.reader.ReadString('\n') //nolint:errcheck
}

// promptAction prompts the user for their decision.
func (r *Reviewer) promptAction() ReviewResult {
	promptColor := color.New(color.FgGreen, color.Bold)
	optionColor := color.New(color.FgWhite)

	promptColor.Print("Action: ")
	optionColor.Print("[a]ccept and build, [v]iew full PKGBUILD, [r]eject and abort: ")

	input, _ := r.reader.ReadString('\n') //nolint:errcheck
	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "a", "accept", "y", "yes", "":
		return ReviewAccept
	case "v", "view":
		return ReviewViewFull
	case "r", "reject", "n", "no", "q", "quit":
		return ReviewReject
	default:
		fmt.Println("Invalid option. Please try again.")
		return r.promptAction()
	}
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// CreateReviewCallback creates a callback function for Builder.OnReview.
func CreateReviewCallback(enabled bool) func(*Package, *PKGBUILD) bool {
	if !enabled {
		return func(*Package, *PKGBUILD) bool {
			return true // Always accept without review
		}
	}

	reviewer := NewReviewer()
	return reviewer.Review
}

// FormatSecuritySummary returns a one-line security summary.
func FormatSecuritySummary(pkgbuild *PKGBUILD) string {
	if pkgbuild.HasDangerousCommands() {
		return fmt.Sprintf("WARNING: %d potentially dangerous command(s) detected",
			len(pkgbuild.DangerousCommands))
	}
	return "No obvious security issues detected"
}

// PrintSecurityReport prints a detailed security report.
func PrintSecurityReport(pkgbuild *PKGBUILD) {
	titleColor := color.New(color.FgCyan, color.Bold)
	errorColor := color.New(color.FgRed, color.Bold)
	warnColor := color.New(color.FgYellow)
	successColor := color.New(color.FgGreen)

	titleColor.Printf("\nSecurity Report: %s\n", pkgbuild.Name())
	fmt.Println(strings.Repeat("-", 50))

	if !pkgbuild.HasDangerousCommands() {
		successColor.Println("No potentially dangerous commands detected.")
		fmt.Println()
		return
	}

	errorColor.Printf("Found %d potentially dangerous command(s):\n\n",
		len(pkgbuild.DangerousCommands))

	for i, cmd := range pkgbuild.DangerousCommands {
		errorColor.Printf("%d. Line %d: %s\n", i+1, cmd.Line, cmd.Reason)
		warnColor.Printf("   Command: %s\n\n", truncate(cmd.Command, 60))
	}
}
