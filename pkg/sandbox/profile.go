// Package sandbox provides sandboxed execution using bubblewrap.
package sandbox

// Profile defines a sandbox configuration for different use cases.
type Profile struct {
	// Name is the profile identifier
	Name string

	// Filesystem bindings
	BindReadOnly  []string // Read-only bind mounts
	BindReadWrite []string // Read-write bind mounts
	DevBinds      []string // Device bind mounts (/dev/null, etc.)
	Tmpfs         []string // Tmpfs mounts

	// Symlinks to create inside the sandbox
	Symlinks map[string]string

	// Process isolation
	UnshareUser   bool // Create new user namespace
	UnsharePID    bool // Create new PID namespace
	UnshareNet    bool // Create new network namespace (no network)
	UnshareIPC    bool // Create new IPC namespace
	UnshareCgroup bool // Create new cgroup namespace

	// User mapping (for user namespace)
	UID int // UID inside sandbox (0 = same as outside)
	GID int // GID inside sandbox (0 = same as outside)

	// Process settings
	DieWithParent bool // Kill sandbox if parent dies
	NewSession    bool // Create new session

	// Environment
	ClearEnv bool              // Clear environment before adding vars
	Env      map[string]string // Environment variables to set
	EnvPass  []string          // Environment variables to pass through

	// Capabilities
	DropCaps []string // Capabilities to drop
}

// ProfileBuild is the default profile for building packages.
// It provides a minimal environment with no network access.
var ProfileBuild = Profile{
	Name: "build",

	BindReadOnly: []string{
		"/usr",
		"/etc/resolv.conf",
		"/etc/passwd",
		"/etc/group",
		"/etc/hosts",
		"/etc/ssl",
		"/etc/ca-certificates",
		"/etc/makepkg.conf",
		"/etc/pacman.conf",
		"/etc/pacman.d",
		"/var/lib/pacman", // Required for pacman dependency checking
	},

	DevBinds: []string{
		"/dev/null",
		"/dev/zero",
		"/dev/random",
		"/dev/urandom",
		"/dev/tty",
		"/dev/fd", // Required for bash process substitution
	},

	Tmpfs: []string{
		"/tmp",
		"/var/tmp",
	},

	Symlinks: map[string]string{
		"/lib":   "/usr/lib",
		"/lib64": "/usr/lib",
		"/bin":   "/usr/bin",
		"/sbin":  "/usr/bin",
	},

	UnshareUser:   false, // Use same user to access files
	UnsharePID:    true,  // Isolate process tree
	UnshareNet:    true,  // No network during build
	UnshareIPC:    true,
	UnshareCgroup: false,

	DieWithParent: true,
	NewSession:    true,

	ClearEnv: false,
	EnvPass: []string{
		"PATH",
		"HOME",
		"USER",
		"LANG",
		"LC_ALL",
		"TERM",
		"MAKEFLAGS",
		"CFLAGS",
		"CXXFLAGS",
		"LDFLAGS",
	},

	Env: map[string]string{
		"SOURCE_DATE_EPOCH": "0",
	},
}

// ProfileFetch is a profile for fetching sources.
// It has network access but limited filesystem access.
var ProfileFetch = Profile{
	Name: "fetch",

	BindReadOnly: []string{
		"/usr",
		"/etc/resolv.conf",
		"/etc/passwd",
		"/etc/group",
		"/etc/hosts",
		"/etc/ssl",
		"/etc/ca-certificates",
	},

	DevBinds: []string{
		"/dev/null",
		"/dev/zero",
		"/dev/random",
		"/dev/urandom",
	},

	Tmpfs: []string{
		"/tmp",
	},

	Symlinks: map[string]string{
		"/lib":   "/usr/lib",
		"/lib64": "/usr/lib",
		"/bin":   "/usr/bin",
		"/sbin":  "/usr/bin",
	},

	UnshareUser:   false,
	UnsharePID:    true,
	UnshareNet:    false, // Allow network for fetching
	UnshareIPC:    true,
	UnshareCgroup: false,

	DieWithParent: true,
	NewSession:    true,

	ClearEnv: false,
	EnvPass: []string{
		"PATH",
		"HOME",
		"USER",
		"LANG",
		"TERM",
		"http_proxy",
		"https_proxy",
		"ftp_proxy",
		"no_proxy",
		"HTTP_PROXY",
		"HTTPS_PROXY",
	},
}

// ProfileMinimal is a highly restricted profile for running untrusted code.
var ProfileMinimal = Profile{
	Name: "minimal",

	BindReadOnly: []string{
		"/usr/bin",
		"/usr/lib",
	},

	DevBinds: []string{
		"/dev/null",
		"/dev/zero",
		"/dev/urandom",
	},

	Tmpfs: []string{
		"/tmp",
	},

	Symlinks: map[string]string{
		"/lib":   "/usr/lib",
		"/lib64": "/usr/lib",
		"/bin":   "/usr/bin",
	},

	UnshareUser:   true,
	UnsharePID:    true,
	UnshareNet:    true,
	UnshareIPC:    true,
	UnshareCgroup: true,

	UID: 65534, // nobody
	GID: 65534, // nobody

	DieWithParent: true,
	NewSession:    true,

	ClearEnv: true,
	Env: map[string]string{
		"PATH": "/usr/bin",
		"HOME": "/tmp",
	},

	DropCaps: []string{
		"ALL",
	},
}

// Clone creates a copy of the profile that can be modified.
func (p Profile) Clone() *Profile {
	clone := p

	// Deep copy slices
	clone.BindReadOnly = append([]string{}, p.BindReadOnly...)
	clone.BindReadWrite = append([]string{}, p.BindReadWrite...)
	clone.DevBinds = append([]string{}, p.DevBinds...)
	clone.Tmpfs = append([]string{}, p.Tmpfs...)
	clone.EnvPass = append([]string{}, p.EnvPass...)
	clone.DropCaps = append([]string{}, p.DropCaps...)

	// Deep copy maps
	if p.Symlinks != nil {
		clone.Symlinks = make(map[string]string)
		for k, v := range p.Symlinks {
			clone.Symlinks[k] = v
		}
	}

	if p.Env != nil {
		clone.Env = make(map[string]string)
		for k, v := range p.Env {
			clone.Env[k] = v
		}
	}

	return &clone
}

// AddBindReadOnly adds a read-only bind mount.
func (p *Profile) AddBindReadOnly(paths ...string) {
	p.BindReadOnly = append(p.BindReadOnly, paths...)
}

// AddBindReadWrite adds a read-write bind mount.
func (p *Profile) AddBindReadWrite(paths ...string) {
	p.BindReadWrite = append(p.BindReadWrite, paths...)
}

// SetEnv sets an environment variable.
func (p *Profile) SetEnv(key, value string) {
	if p.Env == nil {
		p.Env = make(map[string]string)
	}
	p.Env[key] = value
}

// AllowNetwork enables network access.
func (p *Profile) AllowNetwork() {
	p.UnshareNet = false
}

// DenyNetwork disables network access.
func (p *Profile) DenyNetwork() {
	p.UnshareNet = true
}
