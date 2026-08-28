# Spec: Display Package

## ADDED Requirements

### Requirement: Bordered info box helper

The `pkg/display` package SHALL expose `PrintInfoBox` to render a fixed-width bordered box from one or more lines, with an empty string producing a blank separator row. `PrintInfoBox` SHALL compute each line's padding from its visible on-screen width — excluding the bytes of any ANSI color escape sequence or OSC 8 hyperlink escape sequence embedded in the line — so the right border renders in the same column on every row regardless of whether a line contains plain text, a color code, or a hyperlink.

#### Scenario: Plain text line renders with aligned border

- **GIVEN** a caller invokes `display.PrintInfoBox("plain text line")`
- **THEN** the right border renders in the same column as it does for any other line of the same box

#### Scenario: Line containing an OSC 8 hyperlink renders with aligned border

- **GIVEN** a caller invokes `display.PrintInfoBox(line)` where `line` embeds a hyperlink produced by `display.Hyperlink`
- **THEN** the padding is computed from the hyperlink's visible text, not the total byte length including the escape sequence
- **AND** the right border renders in the same column as the other lines of the box

#### Scenario: Blank separator row

- **GIVEN** a caller passes an empty string as one of the lines to `display.PrintInfoBox`
- **THEN** that row renders as a blank, border-only line with no content
