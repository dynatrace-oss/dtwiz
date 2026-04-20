# Display Package

## ADDED Requirements

### Requirement: Shared terminal color definitions

The `pkg/display` package SHALL export a fixed set of terminal color variables used consistently across all CLI output: `ColorOK` (green bold), `ColorError` (red bold), `ColorHeader` (magenta bold), `ColorMuted` (faint), `ColorLabel` (no styling). These SHALL be the canonical colors for all CLI sections — no command or package SHALL define its own equivalent color variables for these roles.

#### Scenario: ColorHeader used for section headings

- **GIVEN** any CLI command prints a section heading
- **WHEN** it calls `display.ColorHeader.Sprint(title)` or `display.Header(title)`
- **THEN** the heading is rendered in magenta bold

#### Scenario: ColorError used for failure states

- **GIVEN** a credential check fails or an operation errors
- **WHEN** the failure message is printed
- **THEN** it is rendered using `display.ColorError` (red bold)

#### Scenario: ColorOK used for success states

- **GIVEN** a credential check passes or an operation succeeds
- **WHEN** the success message is printed
- **THEN** it is rendered using `display.ColorOK` (green bold)

### Requirement: Print helpers for common output patterns

The `pkg/display` package SHALL expose `Header(message string)`, `PrintSectionDivider()`, and `PrintStatusLine(label, message string, c *color.Color)` as helpers for the recurring output patterns used in `dtwiz status` and other commands.

`Header` SHALL print the message indented with two spaces using `ColorHeader`, followed immediately by a section divider — callers SHALL NOT call `PrintSectionDivider()` after `Header()`.
`PrintSectionDivider` SHALL print a 42-character `─` separator indented with two spaces using `ColorMuted`. It is available for use outside of `Header` where a standalone divider is needed.
`PrintStatusLine` SHALL print a line of the form `<label>:  <message>` indented by two spaces, where the label is styled with `ColorLabel` and the message is styled with the provided color.

#### Scenario: Header prints indented magenta bold title followed by a divider

- **GIVEN** a caller invokes `display.Header("Connection Status")`
- **THEN** the output is the text "Connection Status" indented by two spaces, rendered in magenta bold, followed by a newline, followed by a 42-character `─` separator indented by two spaces using `ColorMuted`
- **AND** the caller does not need to call `display.PrintSectionDivider()` separately

#### Scenario: PrintStatusLine formats label and message

- **GIVEN** a caller invokes `display.PrintStatusLine("Environment", "✓ [url.here]", display.ColorOK)`
- **THEN** the output is the label "Environment:" followed by the message "✓ [url.here]" indented by two spaces, with the message in green bold
