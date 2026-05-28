package ui

import (
	"os"

	"golang.org/x/term"
)

// IsInteractive returns true if standard input is attached to a terminal.
// This is used to determine if interactive TUI elements (like huh forms) should be launched.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
