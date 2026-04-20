package display

import (
	"fmt"

	"github.com/fatih/color"
)

func Header(message string) {
	_, err := ColorHeader.Printf("  %s\n", message)
	if err != nil {
		PrintStatusLine("status", fmt.Sprintf("✗ %s", err), ColorError)
	}

	PrintSectionDivider()
}

func PrintSectionDivider() {
	_, err := ColorMuted.Println("  ──────────────────────────────────────────")
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
