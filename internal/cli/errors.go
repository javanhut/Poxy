package cli

import "errors"

var (
	// ErrNoManager is returned when no package manager is available.
	ErrNoManager = errors.New("no package manager detected; specify one with --source")

	// ErrNoPackages is returned when no packages are specified.
	ErrNoPackages = errors.New("no packages specified")

	// ErrSourceNotFound is returned when the specified source is not available.
	ErrSourceNotFound = errors.New("specified package source not found")

	// ErrPackageNotFound is returned when a package cannot be found.
	ErrPackageNotFound = errors.New("package not found")

	// ErrAborted is returned when the user aborts an operation.
	ErrAborted = errors.New("operation aborted by user")
)
