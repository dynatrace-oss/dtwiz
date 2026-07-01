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
		fmt.Fprintf(os.Stderr, "\r\033[2K  %s:  %s", label, message)
	}
}

// ClearPending erases the line written by PrintPending.
func ClearPending() {
	if isTTY() {
		fmt.Fprint(os.Stderr, "\r\033[2K")
	}
}

// Hyperlink returns an OSC 8 hyperlink (ESC \ terminated) when stdout is a TTY,
// falling back to "text (url)" on non-TTY outputs to avoid stray control characters.
func Hyperlink(text, url string) string {
	return hyperlink(text, url, stdoutSupportsHyperlinks())
}

func hyperlink(text, url string, tty bool) string {
	if tty {
		return "\033]8;;" + url + "\033\\" + text + "\033]8;;\033\\"
	}
	return text + ": " + url
}

// PrintInfoBox renders a bordered info box. Pass an empty string to insert a
// blank separator row. Lines must not contain ANSI escape sequences — callers
// are responsible for stripping or avoiding them so padding is correct.
func PrintInfoBox(lines ...string) {
	const boxWidth = 97
	top := "┌" + strings.Repeat("─", boxWidth) + "┐"
	bot := "└" + strings.Repeat("─", boxWidth) + "┘"
	fmt.Println("  " + top)
	for _, line := range lines {
		if line == "" {
			fmt.Println("  │" + strings.Repeat(" ", boxWidth) + "│")
			continue
		}
		spaces := boxWidth - 2 - len([]rune(line))
		if spaces < 0 {
			spaces = 0
		}
		fmt.Println("  │ " + line + strings.Repeat(" ", spaces) + " │")
	}
	fmt.Println("  " + bot)
}

func PrintError(label string, err error) {
	_, _ = fmt.Fprintf(color.Output, "  %s: %s\n", ColorDefault.Sprint(label), ColorError.Sprintf("✗ %s", err))
}

func PrintWarning(label string, err error) {
	_, _ = fmt.Fprintf(color.Error, "  %s: %s\n", ColorDefault.Sprint(label), ColorWarning.Sprintf("⚠ %s", err))
}
