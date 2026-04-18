package detector

// GetFreeBSDManager returns the native package manager for FreeBSD.
func GetFreeBSDManager() string {
	return "pkg"
}

// GetOpenBSDManager returns the native package manager for OpenBSD.
func GetOpenBSDManager() string {
	return "pkg_add"
}
