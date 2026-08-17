# Display Package

## Purpose

Define the shared `pkg/display` package that exports canonical terminal color variables used consistently across all CLI output.

## Requirements

### Requirement: Shared terminal color definitions

The `pkg/display` package SHALL export canonical terminal color variables: `ColorOK` (green bold), `ColorError` (red bold), `ColorWarning` (yellow bold), `ColorBold` (white bold), `ColorHeader` (magenta bold), `ColorMessage` (magenta), `ColorMuted` (faint), `ColorDefault` (no styling). These SHALL be the canonical palette for all CLI output. New colors SHALL only be added to `colors.go` for generic reusable roles; others MUST compose existing `pkg/display` primitives.

#### Scenario: ColorHeader used for section headings

- **GIVEN** any CLI command prints a section heading
- **WHEN** it calls `display.ColorHeader.Sprint(title)` or `display.Header(title)`
- **THEN** the heading is rendered in magenta bold

#### Scenario: ColorMessage used for informational titles

- **GIVEN** any CLI command prints an informational title or inline label (not a section heading)
- **WHEN** it renders the title
- **THEN** it uses `display.ColorMessage` (magenta, no bold)

#### Scenario: ColorError used for failure states

- **GIVEN** a credential check fails or an operation errors
- **WHEN** the failure message is printed
- **THEN** it is rendered using `display.ColorError` (red bold)

#### Scenario: ColorOK used for success states

- **GIVEN** a credential check passes or an operation succeeds
- **WHEN** the success message is printed
- **THEN** it is rendered using `display.ColorOK` (green bold)

#### Scenario: ColorDefault used for unstyled secondary text

- **GIVEN** any CLI command prints secondary or neutral text with no visual emphasis
- **WHEN** it renders that text
- **THEN** it uses `display.ColorDefault` (no styling) — inline `color.New()` SHALL NOT be used

#### Scenario: ColorMuted used for faint/de-emphasized text

- **GIVEN** any CLI command prints de-emphasized text such as hints, cancelled messages, or dry-run notices
- **WHEN** it renders that text
- **THEN** it uses `display.ColorMuted` (faint)

### Requirement: Print helpers for common output patterns

The `pkg/display` package SHALL expose `Header`, `PrintSectionDivider`, `PrintStatusLine`, `PrintFlagLine`, `PrintError`, `PrintPending`, and `ClearPending` as shared helpers for recurring output patterns. `PrintFlagLine` SHALL render `<label>  <message>` without a colon; `PrintError` SHALL render `<label>: ✗ <err>`; `PrintPending` SHALL write a carriage-return-prefixed, non-newline status to stderr only when stderr is a TTY; and `ClearPending` SHALL erase that status and otherwise be a no-op. Callers MUST call `ClearPending` before returning if `PrintPending` was used. Any print pattern recurring across two or more files SHALL be extracted into `pkg/display/print.go`; patterns specific to one file MUST reuse `display.Color*` variables.

#### Scenario: Header prints indented magenta bold title followed by a divider

- **GIVEN** a caller invokes `display.Header("Connection Status")`
- **THEN** the output is the text "Connection Status" indented by two spaces, rendered in magenta bold, followed by a newline, followed by a `─` separator of `DividerLineLength` characters indented by two spaces using `ColorMuted`
- **AND** the caller does not need to call `display.PrintSectionDivider()` separately
- **AND** the caller does not include leading spaces in the message argument

#### Scenario: PrintStatusLine formats label and message

- **GIVEN** a caller invokes `display.PrintStatusLine("Environment", "✓ [url.here]", display.ColorOK)`
- **THEN** the output is the label "Environment:" followed by the message "✓ [url.here]" indented by two spaces, with the message in green bold

#### Scenario: New color or print need evaluated against existing palette

- **GIVEN** a developer needs to print colored output anywhere in the codebase
- **WHEN** they choose a color or styling
- **THEN** they SHALL consult `pkg/display/colors.go` and `pkg/display/print.go` first
- **AND** only introduce a new `color.New(...)` locally if no existing `display.Color*` variable covers the semantic role
- **AND** only add a new variable to `pkg/display` if the role is generic and used in more than one file
