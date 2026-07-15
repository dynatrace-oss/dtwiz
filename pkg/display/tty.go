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
	// Apple Terminal renders OSC 8 as underlined text but doesn't provide a
	// right-click "Open URL" option. Fall back to plain text so its built-in
	// URL detection can make the URL itself right-clickable.
	if os.Getenv("TERM_PROGRAM") == "Apple_Terminal" {
		return false
	}
	// AWS CloudShell doesn't render OSC 8 hyperlinks.
	if os.Getenv("AWS_EXECUTION_ENV") == "CloudShell" {
		return false
	}
	return true
}

// StdoutSupportsHyperlinks reports whether stdout supports OSC 8 hyperlinks.
func StdoutSupportsHyperlinks() bool {
	return stdoutSupportsHyperlinks()
}
