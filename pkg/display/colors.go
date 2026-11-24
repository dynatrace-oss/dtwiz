package display

import (
	"github.com/fatih/color"
)

var (
	ColorOK      = color.New(color.FgGreen, color.Bold)
	ColorError   = color.New(color.FgRed, color.Bold)
	ColorWarning = color.New(color.FgYellow, color.Bold)
	ColorBold    = color.New(color.FgWhite, color.Bold)
	// ColorEmphasis is bold in the terminal's own foreground color, so it stays
	// readable on both dark and light backgrounds (unlike ColorBold's fixed white).
	ColorEmphasis = color.New(color.Bold)
	ColorDefault  = color.New()
	ColorMuted    = color.New(color.Faint)
	ColorHeader   = color.New(color.FgMagenta, color.Bold)
	ColorMessage  = color.New(color.FgMagenta)
)
