# Display Package

## ADDED Requirements

### Requirement: Shared terminal color definitions

The `pkg/display` package SHALL export a fixed set of terminal color variables used consistently across all CLI output: `ColorOK` (green bold), `ColorError` (red bold), `ColorWarning` (yellow bold), `ColorBold` (white bold), `ColorHeader` (magenta bold), `ColorMessage` (magenta), `ColorMuted` (faint), `ColorDefault` (no styling). These SHALL be the canonical colors for all CLI sections — no command or package SHALL define its own equivalent color variables for these roles.

Any new color or print need SHALL be evaluated against this palette first. A new color variable SHALL only be added to `pkg/display/colors.go` if the role it represents is generic and reusable across multiple commands or packages. Color variables that are too specific to a single command or feature SHALL NOT be added to `pkg/display` — they SHALL be defined locally where needed, but MUST be composed from `pkg/display` primitives (e.g. `color.New(...)` is only acceptable if no existing `display.Color*` variable covers the role).

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

The `pkg/display` package SHALL expose `Header(message string)`, `PrintSectionDivider()`, `PrintStatusLine(label, message string, c *color.Color)`, `PrintFlagLine(label, message string, c *color.Color)`, and `PrintError(label string, err error)` as helpers for recurring output patterns used across commands and installers.

`Header` SHALL print the message indented with two spaces using `ColorHeader`, followed immediately by a section divider — callers SHALL NOT call `PrintSectionDivider()` after `Header()`. Callers SHALL NOT add leading spaces to the message argument; `Header` applies indentation itself.
`PrintSectionDivider` SHALL print a `─` separator of `DividerLineLength` characters indented with two spaces using `ColorMuted`. It is available for use outside of `Header` where a standalone divider is needed.
`PrintStatusLine` SHALL print a line of the form `<label>:  <message>` (indented two spaces) where the label is styled with `ColorDefault` and the message is styled with the provided color.
`PrintFlagLine` SHALL print a line of the form `<label>  <message>` (no colon, indented two spaces) where the label is styled with `ColorDefault` and the message is styled with the provided color.
`PrintError` SHALL print a line of the form `<label>: ✗ <err>` (indented two spaces) where the error text is styled with `ColorError`.

Any print pattern that recurs across two or more files SHALL be extracted into `pkg/display/print.go`. Print patterns that are specific to a single installer or command MAY remain in that file but MUST reuse `display.Color*` variables and MUST NOT construct their own `color.New(...)` instances for roles already covered by the palette.

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
