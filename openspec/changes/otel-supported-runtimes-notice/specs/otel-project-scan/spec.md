## ADDED Requirements

### Requirement: Supported-runtimes notice shown before project selection

When `dtwiz install otel` runs without `--project` and one or more projects are detected, the interactive flow SHALL display a notice — after the detected-projects list and before the `Select a project to instrument` prompt — stating that Python, Java, and Node.js projects are auto-instrumented, and pointing to the OpenTelemetry walkthroughs documentation for other runtimes. The notice SHALL be shown once per install run and SHALL NOT be repeated if the user retries project selection after choosing a project that can't be auto-instrumented.

#### Scenario: Notice shown on first project listing

- **GIVEN** the interactive project scan detects at least one project
- **WHEN** the detected-projects list is printed
- **THEN** the supported-runtimes notice is displayed immediately after the list and before the selection prompt

#### Scenario: Notice not repeated on reselection

- **GIVEN** the supported-runtimes notice has already been shown once in the current install run
- **WHEN** the user selects a project that can't be auto-instrumented and chooses to select another
- **AND** the detected-projects list is printed again
- **THEN** the supported-runtimes notice is NOT shown a second time

#### Scenario: Notice links to walkthroughs documentation

- **GIVEN** the supported-runtimes notice is displayed
- **WHEN** the terminal supports OSC 8 hyperlinks
- **THEN** the link to the OpenTelemetry walkthroughs documentation renders as a clickable hyperlink
- **AND** WHEN the terminal does not support OSC 8 hyperlinks, the plain documentation URL is shown instead
