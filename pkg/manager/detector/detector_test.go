package detector

import (
	"runtime"
	"testing"
)

func TestOSType(t *testing.T) {
	tests := []struct {
		name     string
		osType   OSType
		expected string
	}{
		{"Linux", OSLinux, "linux"},
		{"Darwin", OSDarwin, "darwin"},
		{"Windows", OSWindows, "windows"},
		{"Unknown", OSUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.osType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.osType)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	info, err := Detect()
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if info == nil {
		t.Fatal("Detect() returned nil")
	}

	// Check that arch matches runtime
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected Arch '%s', got '%s'", runtime.GOARCH, info.Arch)
	}

	// Check that OS is detected
	switch runtime.GOOS {
	case "linux":
		if info.OS != OSLinux {
			t.Errorf("expected OS Linux, got %s", info.OS)
		}
	case "darwin":
		if info.OS != OSDarwin {
			t.Errorf("expected OS Darwin, got %s", info.OS)
		}
	case "windows":
		if info.OS != OSWindows {
			t.Errorf("expected OS Windows, got %s", info.OS)
		}
	}
}

func TestSystemInfo_MatchesDistro(t *testing.T) {
	info := &SystemInfo{
		OS:           OSLinux,
		Distribution: "ubuntu",
		DistroFamily: []string{"debian"},
	}

	tests := []struct {
		distros  []string
		expected bool
	}{
		{[]string{"ubuntu"}, true},
		{[]string{"debian"}, true},
		{[]string{"fedora"}, false},
		{[]string{"arch", "ubuntu"}, true},
		{[]string{"fedora", "rhel"}, false},
	}

	for _, tt := range tests {
		result := info.MatchesDistro(tt.distros...)
		if result != tt.expected {
			t.Errorf("MatchesDistro(%v) = %v, want %v", tt.distros, result, tt.expected)
		}
	}
}

func TestSystemInfo_OSChecks(t *testing.T) {
	linuxInfo := &SystemInfo{OS: OSLinux}
	darwinInfo := &SystemInfo{OS: OSDarwin}
	windowsInfo := &SystemInfo{OS: OSWindows}

	if !linuxInfo.IsLinux() {
		t.Error("IsLinux() should return true for Linux")
	}
	if linuxInfo.IsDarwin() {
		t.Error("IsDarwin() should return false for Linux")
	}

	if !darwinInfo.IsDarwin() {
		t.Error("IsDarwin() should return true for Darwin")
	}

	if !windowsInfo.IsWindows() {
		t.Error("IsWindows() should return true for Windows")
	}
}

func TestGetNativeManager(t *testing.T) {
	tests := []struct {
		distro   string
		expected string
	}{
		{"ubuntu", "apt"},
		{"debian", "apt"},
		{"fedora", "dnf"},
		{"arch", "pacman"},
		{"manjaro", "pacman"},
		{"opensuse", "zypper"},
		{"void", "xbps"},
		{"alpine", "apk"},
		{"gentoo", "emerge"},
		{"solus", "eopkg"},
		{"nixos", "nix"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.distro, func(t *testing.T) {
			result := GetNativeManager(tt.distro)
			if result != tt.expected {
				t.Errorf("GetNativeManager(%s) = %s, want %s", tt.distro, result, tt.expected)
			}
		})
	}
}

func TestGetNativeManagerForFamily(t *testing.T) {
	tests := []struct {
		distro   string
		idLike   []string
		expected string
	}{
		{"linuxmint", []string{"ubuntu", "debian"}, "apt"},
		{"pop", []string{"ubuntu", "debian"}, "apt"},
		{"rocky", []string{"rhel", "fedora"}, "dnf"},
		{"endeavouros", []string{"arch"}, "pacman"},
		{"unknown", []string{"alsounknown"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.distro, func(t *testing.T) {
			result := GetNativeManagerForFamily(tt.distro, tt.idLike)
			if result != tt.expected {
				t.Errorf("GetNativeManagerForFamily(%s, %v) = %s, want %s",
					tt.distro, tt.idLike, result, tt.expected)
			}
		})
	}
}
