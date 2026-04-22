package display

import (
	"github.com/fatih/color"
)

var (
	ColorOK     = color.New(color.FgGreen, color.Bold)
	ColorError  = color.New(color.FgRed, color.Bold)
	ColorLabel  = color.New()
	ColorMuted  = color.New(color.Faint)
	ColorHeader = color.New(color.FgMagenta, color.Bold)
)
