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

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [packages...]",
	Short: "Upgrade installed packages",
	Long: `Upgrade installed packages to their latest versions.

If no packages are specified, all installed packages will be upgraded.

Examples:
  poxy upgrade              # Upgrade all packages
  poxy upgrade vim git      # Upgrade specific packages
  poxy upgrade -y           # Upgrade all without confirmation`,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get package manager
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// Resolve aliases if specific packages given
	packages := resolvePackages(args)

	if len(packages) > 0 {
		ui.InfoMsg("Upgrading %d package(s) using %s", len(packages), mgr.DisplayName())
		for _, pkg := range packages {
			ui.MutedMsg("  - %s", pkg)
		}
	} else {
		ui.InfoMsg("Upgrading all packages using %s", mgr.DisplayName())
	}

	// Confirm if not auto-confirmed
	if !cfg.General.AutoConfirm && !cfg.General.DryRun {
		confirmed, err := ui.Confirm("Proceed with upgrade?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Capture pre-operation snapshot
	capturePreOperationSnapshot(ctx, snapshot.TriggerUpgrade, packages)

	// Create history entry
	entry := history.NewEntry(history.OpUpgrade, mgr.Name(), packages)

	// Capture PM output so we can show a clean summary.
	var buf bytes.Buffer
	opts := manager.UpgradeOpts{
		AutoConfirm: true, // poxy already confirmed above
		DryRun:      cfg.General.DryRun,
		Packages:    packages,
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

	err = runUpgradeWithSpinner(ctx, mgr, packages, opts)

	if err != nil {
		entry.MarkFailed(err)
		emitUpgradeFailure(buf.String(), err)
	} else {
		entry.MarkSuccess()
		emitUpgradeSuccess(buf.String(), packages, mgr.DisplayName())
	}

	recordInstallHistory(entry)
	return err
}

// runUpgradeWithSpinner wraps mgr.Upgrade with a spinner for consistent
// appearance. Verbose and AUR paths skip the spinner.
func runUpgradeWithSpinner(ctx context.Context, mgr manager.Manager, packages []string, opts manager.UpgradeOpts) error {
	useSpinner := !cfg.Output.Verbose && mgr.Type() != manager.TypeAUR

	var msg string
	if len(packages) > 0 {
		msg = fmt.Sprintf("Upgrading %s via %s", strings.Join(packages, " "), mgr.DisplayName())
	} else {
		msg = fmt.Sprintf("Upgrading all packages via %s", mgr.DisplayName())
	}

	if !useSpinner {
		ui.InfoMsg("%s…", msg)
		return mgr.Upgrade(ctx, opts)
	}

	sp := ui.NewSpinner(msg + "…")
	sp.Start()
	err := mgr.Upgrade(ctx, opts)
	sp.Stop()
	return err
}

func emitUpgradeSuccess(captured string, packages []string, displayName string) {
	summary := ui.ParsePackageSummary(captured)
	var prefix string
	if len(packages) > 0 {
		prefix = strings.Join(packages, " ")
	} else {
		prefix = "packages"
	}
	if summary.VersionTag != "" {
		prefix = summary.VersionTag
	}
	if summary.Size != "" {
		ui.SuccessMsg("Upgraded %s (%s) on %s", prefix, summary.Size, displayName)
	} else {
		ui.SuccessMsg("Upgraded %s on %s", prefix, displayName)
	}
}

func emitUpgradeFailure(captured string, err error) {
	if !cfg.Output.Verbose && captured != "" {
		fmt.Fprint(os.Stderr, captured)
		if !strings.HasSuffix(captured, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	ui.ErrorMsg("Upgrade failed: %v", err)
}
