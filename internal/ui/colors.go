// Package ui provides terminal UI helpers for poxy.
package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	// Colors for different message types
	Success = color.New(color.FgGreen, color.Bold)
	Error   = color.New(color.FgRed, color.Bold)
	Warning = color.New(color.FgYellow, color.Bold)
	Info    = color.New(color.FgCyan)
	Header  = color.New(color.FgMagenta, color.Bold)
	Muted   = color.New(color.FgHiBlack)

	// Colors for specific elements
	PackageName    = color.New(color.FgWhite, color.Bold)
	PackageVersion = color.New(color.FgGreen)
	PackageSource  = color.New(color.FgCyan)
	Installed      = color.New(color.FgGreen)
	NotInstalled   = color.New(color.FgHiBlack)
)

// UseColors represents whether colors should be used.
var UseColors = true

// UseUnicode represents whether unicode symbols should be used.
var UseUnicode = true

// Symbols for status indicators
var (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "!"
	SymbolInfo    = "→"
	SymbolPending = "○"
	SymbolArrow   = "→"
)

// Init initializes the UI settings based on configuration.
func Init(useColors, useUnicode bool) {
	UseColors = useColors
	UseUnicode = useUnicode

	if !useColors || os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}

	if !useUnicode {
		SymbolSuccess = "[OK]"
		SymbolError = "[ERROR]"
		SymbolWarning = "[WARN]"
		SymbolInfo = "->"
		SymbolPending = "[ ]"
		SymbolArrow = "->"
	}
}

// SuccessMsg prints a success message.
func SuccessMsg(format string, args ...interface{}) {
	Success.Printf(SymbolSuccess+" "+format+"\n", args...)
}

// ErrorMsg prints an error message.
func ErrorMsg(format string, args ...interface{}) {
	Error.Printf(SymbolError+" "+format+"\n", args...)
}

// WarningMsg prints a warning message.
func WarningMsg(format string, args ...interface{}) {
	Warning.Printf(SymbolWarning+" "+format+"\n", args...)
}

// InfoMsg prints an info message.
func InfoMsg(format string, args ...interface{}) {
	Info.Printf(SymbolInfo+" "+format+"\n", args...)
}

// HeaderMsg prints a header message.
func HeaderMsg(format string, args ...interface{}) {
	Header.Printf("\n"+format+"\n", args...)
}

// MutedMsg prints a muted (dim) message.
func MutedMsg(format string, args ...interface{}) {
	Muted.Printf(format+"\n", args...)
}

// Println prints a plain line with formatting.
func Println(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Bold returns a bold string.
func Bold(s string) string {
	return color.New(color.Bold).Sprint(s)
}

// Green returns a green string.
func Green(s string) string {
	return color.GreenString(s)
}

// Red returns a red string.
func Red(s string) string {
	return color.RedString(s)
}

// Yellow returns a yellow string.
func Yellow(s string) string {
	return color.YellowString(s)
}

// Cyan returns a cyan string.
func Cyan(s string) string {
	return color.CyanString(s)
}

// Magenta returns a magenta string.
func Magenta(s string) string {
	return color.MagentaString(s)
}
