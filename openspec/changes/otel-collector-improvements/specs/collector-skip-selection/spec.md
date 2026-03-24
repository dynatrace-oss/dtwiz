## ADDED Requirements

### Requirement: User can skip app-type selection
The collector setup SHALL offer a "None" option in the app-type selection menu, allowing the user to install the collector without triggering any auto-instrumentation.

#### Scenario: User selects None
- **WHEN** the user selects "None — collector only" from the app-type menu
- **THEN** the collector SHALL be installed without invoking any language-specific instrumentation installer

#### Scenario: None option is always present
- **WHEN** the app-type selection menu is displayed
- **THEN** a "None — collector only" option SHALL always be present regardless of which runtimes are detected
