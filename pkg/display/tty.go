package display

import (
	"os"

	"golang.org/x/term"
)

func isTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// stdoutSupportsHyperlinks reports whether stdout is an interactive terminal
// that renders OSC 8 hyperlinks. macOS Terminal.app silently drops OSC 8
// sequences, which would hide the URL entirely, so it is excluded.
func stdoutSupportsHyperlinks() bool {
	if !isStdoutTTY() {
		return false
	}
	return os.Getenv("TERM_PROGRAM") != "Apple_Terminal"
}
