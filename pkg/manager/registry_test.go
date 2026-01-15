package manager

import (
	"context"
	"testing"

	"poxy/internal/config"
)

// MockManager for testing
type MockManager struct {
	name        string
	displayName string
	mgrType     ManagerType
	available   bool
	needsSudo   bool
}

func (m *MockManager) Name() string        { return m.name }
func (m *MockManager) DisplayName() string { return m.displayName }
func (m *MockManager) Type() ManagerType   { return m.mgrType }
func (m *MockManager) IsAvailable() bool   { return m.available }
func (m *MockManager) NeedsSudo() bool     { return m.needsSudo }

func (m *MockManager) Install(_ context.Context, _ []string, _ InstallOpts) error     { return nil }
func (m *MockManager) Uninstall(_ context.Context, _ []string, _ UninstallOpts) error { return nil }
func (m *MockManager) Update(_ context.Context) error                                 { return nil }
func (m *MockManager) Upgrade(_ context.Context, _ UpgradeOpts) error                 { return nil }
func (m *MockManager) Search(_ context.Context, _ string, _ SearchOpts) ([]Package, error) {
	return nil, nil
}
func (m *MockManager) Info(_ context.Context, _ string) (*PackageInfo, error) { return nil, nil }
func (m *MockManager) ListInstalled(_ context.Context, _ ListOpts) ([]Package, error) {
	return nil, nil
}
func (m *MockManager) IsInstalled(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *MockManager) Clean(_ context.Context, _ CleanOpts) error            { return nil }
func (m *MockManager) Autoremove(_ context.Context) error                    { return nil }

func TestNewRegistry(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegistryRegister(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	mock := &MockManager{
		name:        "mock",
		displayName: "Mock Manager",
		mgrType:     TypeNative,
		available:   true,
	}

	registry.Register(mock)

	mgr, ok := registry.Get("mock")
	if !ok {
		t.Error("Get() should find registered manager")
	}
	if mgr != mock {
		t.Error("Get() returned wrong manager")
	}
}

func TestRegistryGet(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	// Non-existent manager
	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent manager")
	}
}

func TestRegistryAvailable(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	// Register some managers
	available := &MockManager{name: "available", available: true, mgrType: TypeNative}
	unavailable := &MockManager{name: "unavailable", available: false, mgrType: TypeNative}

	registry.Register(available)
	registry.Register(unavailable)

	managers := registry.Available()

	// Should only contain the available manager
	found := false
	for _, mgr := range managers {
		if mgr.Name() == "available" {
			found = true
		}
		if mgr.Name() == "unavailable" {
			t.Error("Available() should not include unavailable managers")
		}
	}

	if !found {
		t.Error("Available() should include available managers")
	}
}

func TestRegistryAvailableByType(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	native := &MockManager{name: "native", mgrType: TypeNative, available: true}
	universal := &MockManager{name: "universal", mgrType: TypeUniversal, available: true}
	aur := &MockManager{name: "aur", mgrType: TypeAUR, available: true}

	registry.Register(native)
	registry.Register(universal)
	registry.Register(aur)

	// Get native managers
	nativeManagers := registry.AvailableByType(TypeNative)
	if len(nativeManagers) != 1 || nativeManagers[0].Name() != "native" {
		t.Error("AvailableByType(TypeNative) returned wrong results")
	}

	// Get universal managers
	universalManagers := registry.AvailableByType(TypeUniversal)
	if len(universalManagers) != 1 || universalManagers[0].Name() != "universal" {
		t.Error("AvailableByType(TypeUniversal) returned wrong results")
	}

	// Get AUR managers
	aurManagers := registry.AvailableByType(TypeAUR)
	if len(aurManagers) != 1 || aurManagers[0].Name() != "aur" {
		t.Error("AvailableByType(TypeAUR) returned wrong results")
	}
}

func TestRegistryAll(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	mock1 := &MockManager{name: "mock1", available: true}
	mock2 := &MockManager{name: "mock2", available: false}

	registry.Register(mock1)
	registry.Register(mock2)

	all := registry.All()
	if len(all) != 2 {
		t.Errorf("All() should return 2 managers, got %d", len(all))
	}
}

func TestRegistryGetManagerForSource(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	mock := &MockManager{name: "apt", mgrType: TypeNative, available: true}
	registry.Register(mock)

	// Get by name
	mgr, err := registry.GetManagerForSource("apt")
	if err != nil {
		t.Errorf("GetManagerForSource('apt') error: %v", err)
	}
	if mgr.Name() != "apt" {
		t.Error("GetManagerForSource() returned wrong manager")
	}

	// Get non-existent
	_, err = registry.GetManagerForSource("nonexistent")
	if err == nil {
		t.Error("GetManagerForSource() should error for non-existent source")
	}
}

func TestRegistryNative(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	// Before detection, native should be nil
	if registry.Native() != nil {
		t.Error("Native() should be nil before detection")
	}
}

func TestRegistrySystemInfo(t *testing.T) {
	cfg := config.Default()
	registry := NewRegistry(cfg)

	// Before detection
	if registry.SystemInfo() != nil {
		t.Error("SystemInfo() should be nil before detection")
	}

	// After detection
	err := registry.Detect()
	if err != nil {
		t.Logf("Detect() returned error (may be expected): %v", err)
	}

	sysInfo := registry.SystemInfo()
	if sysInfo == nil {
		t.Error("SystemInfo() should not be nil after detection")
	}
}
