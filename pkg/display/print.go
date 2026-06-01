package display

import (
	"fmt"
	"os"
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

// PrintPending writes an in-progress status to stderr using \r so a subsequent
// ClearPending or PrintStatusLine starts on a clean line. No-ops when stderr
// is not a TTY to avoid polluting CI logs.
func PrintPending(label, message string) {
	if isTTY() {
		fmt.Fprintf(os.Stderr, "\r  %s:  %s", label, message)
	}
}

// ClearPending erases the line written by PrintPending.
func ClearPending() {
	if isTTY() {
		fmt.Fprint(os.Stderr, "\r\033[2K")
	}
}

func PrintError(label string, err error) {
	_, _ = fmt.Fprintf(color.Output, "  %s: %s\n", ColorDefault.Sprint(label), ColorError.Sprintf("✗ %s", err))
}
