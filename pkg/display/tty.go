package display

import (
	"os"

	"golang.org/x/term"
)

func isTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
