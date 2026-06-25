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

func stdoutSupportsHyperlinks() bool {
	if !isStdoutTTY() {
		return false
	}
	// Terminal.app underlines OSC 8 text but doesn't make it clickable.
	return os.Getenv("TERM_PROGRAM") != "Apple_Terminal"
}
