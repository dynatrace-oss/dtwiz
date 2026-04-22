package display

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	DividerLineLength int = 42
)

func Header(label string, message string) {
	_, err := ColorHeader.Printf("  %s\n", message)
	if err != nil {
		PrintStatusLine(label, fmt.Sprintf("✗ %s", err), ColorError)
	}

	PrintSectionDivider(label)
}

func PrintSectionDivider(label string) {
	_, err := ColorMuted.Println("  " + strings.Repeat("─", DividerLineLength))
	if err != nil {
		PrintError(label, err)
	}
}

func PrintStatusLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s:  %s\n", ColorLabel.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		PrintError(label, err)
	}
}

// PrintFlagLine prints a feature flag line without a colon after the label,
// producing output like:  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)
func PrintFlagLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s  %s\n", ColorLabel.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		PrintError(label, err)
	}
}

func PrintError(label string, err error) {
	fmt.Printf("  %s: %s\n", label, ColorError.Sprintf("✗ %s", err))
}
