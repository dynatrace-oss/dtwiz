package display

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	DividerLineLength int = 50
	DefaultLabel          = "dtwiz"
)

func Header(message string) {
	_, err := ColorHeader.Printf("  %s\n", message)
	if err != nil {
		PrintError(DefaultLabel, err)
	}

	PrintSectionDivider()
}

func PrintSectionDivider() {
	_, err := ColorMuted.Println("  " + strings.Repeat("─", DividerLineLength))
	if err != nil {
		PrintError(DefaultLabel, err)
	}
}

func PrintStatusLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s:  %s\n", ColorDefault.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		PrintError(label, err)
	}
}

// PrintFlagLine prints a feature flag line without a colon after the label,
// producing output like:  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)
func PrintFlagLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s  %s\n", ColorDefault.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		PrintError(label, err)
	}
}

func PrintError(label string, err error) {
	fmt.Printf("  %s: %s\n", label, ColorError.Sprintf("✗ %s", err))
}
