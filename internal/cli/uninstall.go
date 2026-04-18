package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"poxy/internal/executor"
	"poxy/internal/history"
	"poxy/internal/ui"
	"poxy/pkg/manager"
	"poxy/pkg/snapshot"

	"github.com/spf13/cobra"
)

var (
	uninstallPurge     bool
	uninstallRecursive bool
)

var uninstallCmd = &cobra.Command{
	Use:     "uninstall [packages...]",
	Aliases: []string{"remove", "rm"},
	Short:   "Remove one or more packages",
	Long: `Remove packages using the detected system package manager
or a specified source.

Examples:
  poxy uninstall vim                # Remove package
  poxy uninstall -y firefox         # Remove without confirmation
  poxy uninstall --purge nginx      # Remove including config files
  poxy uninstall -r package         # Remove with unused dependencies`,
	Args: cobra.MinimumNArgs(1),
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "remove configuration files too")
	uninstallCmd.Flags().BoolVarP(&uninstallRecursive, "recursive", "r", false, "remove unused dependencies")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// Resolve aliases
	packages := resolvePackages(args)

	// Show what we're doing
	ui.InfoMsg("Removing %d package(s) using %s", len(packages), mgr.DisplayName())
	for _, pkg := range packages {
		ui.MutedMsg("  - %s", pkg)
	}

	if uninstallPurge {
		ui.WarningMsg("Configuration files will also be removed")
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with removal?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Capture pre-operation snapshot
	capturePreOperationSnapshot(ctx, snapshot.TriggerUninstall, packages)

	// Create history entry
	entry := history.NewEntry(history.OpUninstall, mgr.Name(), packages)

	// Capture PM output so we can show a clean summary (and surface it on failure).
	var buf bytes.Buffer
	opts := manager.UninstallOpts{
		AutoConfirm: true, // poxy already confirmed above
		DryRun:      cfg.General.DryRun,
		Purge:       uninstallPurge,
		Recursive:   uninstallRecursive,
		OutputSink:  &buf,
	}

	// Pre-warm sudo so password prompts don't get swallowed by captured exec.
	if mgr.NeedsSudo() {
		if sudoErr := executor.New(false, false).EnsureSudo(ctx); sudoErr != nil {
			entry.MarkFailed(sudoErr)
			recordInstallHistory(entry)
			ui.ErrorMsg("Could not escalate privileges: %v", sudoErr)
			return sudoErr
		}
	}

	err = runUninstallWithSpinner(ctx, mgr, packages, opts)

	if err != nil {
		entry.MarkFailed(err)
		emitRemovalFailure(buf.String(), err)
	} else {
		entry.MarkSuccess()
		emitRemovalSuccess(buf.String(), packages, mgr.DisplayName())
	}

	recordInstallHistory(entry)
	return err
}

// runUninstallWithSpinner wraps mgr.Uninstall with a spinner for a consistent
// appearance. Verbose and AUR paths skip the spinner.
func runUninstallWithSpinner(ctx context.Context, mgr manager.Manager, packages []string, opts manager.UninstallOpts) error {
	useSpinner := !cfg.Output.Verbose && mgr.Type() != manager.TypeAUR
	msg := fmt.Sprintf("Removing %s via %s", strings.Join(packages, " "), mgr.DisplayName())

	if !useSpinner {
		ui.InfoMsg("%s…", msg)
		return mgr.Uninstall(ctx, packages, opts)
	}

	sp := ui.NewSpinner(msg + "…")
	sp.Start()
	err := mgr.Uninstall(ctx, packages, opts)
	sp.Stop()
	return err
}

func emitRemovalSuccess(captured string, packages []string, displayName string) {
	summary := ui.ParsePackageSummary(captured)
	label := strings.Join(packages, " ")
	if summary.VersionTag != "" {
		label = summary.VersionTag
	}
	if summary.Size != "" {
		ui.SuccessMsg("Removed %s (%s) from %s", label, summary.Size, displayName)
	} else {
		ui.SuccessMsg("Removed %s from %s", label, displayName)
	}
}

func emitRemovalFailure(captured string, err error) {
	if !cfg.Output.Verbose && captured != "" {
		fmt.Fprint(os.Stderr, captured)
		if !strings.HasSuffix(captured, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	ui.ErrorMsg("Removal failed: %v", err)
}
