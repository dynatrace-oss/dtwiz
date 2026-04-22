package display

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	DividerLineLength int = 42
)

func Header(message string) {
	_, err := ColorHeader.Printf("  %s\n", message)
	if err != nil {
		PrintStatusLine("status", fmt.Sprintf("✗ %s", err), ColorError)
	}

	PrintSectionDivider()
}

func PrintSectionDivider() {
	_, err := ColorMuted.Println("  " + strings.Repeat("─", DividerLineLength))
	if err != nil {
		PrintStatusLine("status", fmt.Sprintf("✗ %s", err), ColorError)
	}
}

func PrintStatusLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s:  %s\n", ColorLabel.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		fmt.Printf("status: %s\n", ColorError.Sprintf("✗ %s", err))
	}
}

// PrintFlagLine prints a feature flag line without a colon after the label,
// producing output like:  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)
func PrintFlagLine(label, message string, colorFunc *color.Color) {
	_, err := fmt.Fprintf(color.Output, "  %s  %s\n", ColorLabel.Sprint(label), colorFunc.Sprint(message))
	if err != nil {
		fmt.Printf("status: %s\n", ColorError.Sprintf("✗ %s", err))
	}
}
