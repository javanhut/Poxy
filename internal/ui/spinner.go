package ui

import (
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps the spinner library for consistent styling.
type Spinner struct {
	s *spinner.Spinner
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	charSet := spinner.CharSets[14] // ⣾⣽⣻⢿⡿⣟⣯⣷
	if !UseUnicode {
		charSet = spinner.CharSets[0] // |/-\
	}

	s := spinner.New(charSet, 100*time.Millisecond)
	s.Suffix = " " + message

	if UseColors {
		s.Color("cyan")
	}

	return &Spinner{s: s}
}

// Start starts the spinner.
func (sp *Spinner) Start() {
	sp.s.Start()
}

// Stop stops the spinner.
func (sp *Spinner) Stop() {
	sp.s.Stop()
}

// Success stops the spinner with a success message.
func (sp *Spinner) Success(message string) {
	sp.s.Stop()
	SuccessMsg(message)
}

// Error stops the spinner with an error message.
func (sp *Spinner) Error(message string) {
	sp.s.Stop()
	ErrorMsg(message)
}

// UpdateMessage updates the spinner message.
func (sp *Spinner) UpdateMessage(message string) {
	sp.s.Suffix = " " + message
}

// WithSpinner runs a function with a spinner, showing success or error on completion.
func WithSpinner(message string, fn func() error) error {
	sp := NewSpinner(message)
	sp.Start()

	err := fn()

	if err != nil {
		sp.Error(err.Error())
		return err
	}

	sp.Success(message + " - done")
	return nil
}
