package universal

import (
	"testing"

	"poxy/pkg/manager"
)

func TestNativeAURBuildOptionsUsesConfiguredSandboxSetting(t *testing.T) {
	mgr := NewNativeAUR(true, false)
	opts := mgr.buildOptions(manager.InstallOpts{AutoConfirm: false})

	if opts.UseSandbox {
		t.Fatal("expected UseSandbox=false from manager config")
	}
	if !opts.ReviewPKGBUILD {
		t.Fatal("expected ReviewPKGBUILD=true when interactive review is enabled")
	}
	if opts.OnReview == nil {
		t.Fatal("expected OnReview callback when review is enabled")
	}
}

func TestNativeAURBuildOptionsDisableReviewInAutoConfirmMode(t *testing.T) {
	mgr := NewNativeAUR(true, true)
	opts := mgr.buildOptions(manager.InstallOpts{AutoConfirm: true})

	if !opts.UseSandbox {
		t.Fatal("expected UseSandbox=true from manager config")
	}
	if opts.ReviewPKGBUILD {
		t.Fatal("expected ReviewPKGBUILD=false in auto-confirm mode")
	}
	if opts.OnReview != nil {
		t.Fatal("expected OnReview callback to be nil in auto-confirm mode")
	}
}
