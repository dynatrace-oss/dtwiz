# Spec: RUM Manual Instructions

## ADDED Requirements

### Requirement: Print the RUM JavaScript tag for manual insertion

The system SHALL render the RUM JavaScript tag together with instructions for inserting it manually, following the Dynatrace agentless monitoring guide. The tag SHALL be printed on a line of its own, with no indentation, color, border, or other characters added, so that the whole line can be copied into the application unchanged.

#### Scenario: Tag and instructions are rendered

- **GIVEN** a RUM JavaScript tag
- **WHEN** the manual instructions are rendered
- **THEN** the output starts with a "Real User Monitoring" section heading
- **AND** it asks the user to add the tag to their app manually
- **AND** it contains the tag on its own line, exactly as given and with nothing before or after it on that line
- **AND** it tells the user to place the tag in the `<head>` of every page to monitor, as the first script, and to use a shared layout or template if the app has one
- **AND** it tells the user that data appears in Experience Vitals within about ten minutes once the app receives traffic
- **AND** it links to the Dynatrace Real User Monitoring documentation

#### Scenario: Surrounding whitespace in the tag is removed

- **GIVEN** a RUM JavaScript tag with leading or trailing whitespace or newlines
- **WHEN** the manual instructions are rendered
- **THEN** the tag line contains the tag without that whitespace

#### Scenario: Tag line is never colored

- **GIVEN** a terminal that supports colors
- **WHEN** the manual instructions are rendered
- **THEN** the tag line contains no color escape sequences

#### Scenario: Empty tag

- **GIVEN** a RUM JavaScript tag that is empty or contains only whitespace
- **WHEN** the manual instructions are rendered
- **THEN** nothing is printed
- **AND** an error is returned stating that the tag is missing

### Requirement: Explain why manual insertion is needed

The system SHALL accept an optional reason explaining why automatic injection is not possible, and SHALL show it only when it is provided.

#### Scenario: Reason provided

- **GIVEN** a RUM JavaScript tag and the reason "Next.js detected"
- **WHEN** the manual instructions are rendered
- **THEN** the output states that automatic injection is not possible because of "Next.js detected", before the tag

#### Scenario: No reason provided

- **GIVEN** a RUM JavaScript tag and no reason
- **WHEN** the manual instructions are rendered
- **THEN** the output contains no line about automatic injection being impossible
- **AND** all other lines are rendered unchanged

### Requirement: Show a framework-specific placement hint

The system SHALL accept an optional hint describing where to place the tag in the user's framework, and SHALL show it only when it is provided. The content of the hint is supplied by the caller and rendered as given.

#### Scenario: Hint provided

- **GIVEN** a RUM JavaScript tag and the hint "Next.js: add it to app/layout.tsx"
- **WHEN** the manual instructions are rendered
- **THEN** the output contains the hint next to the placement instructions

#### Scenario: No hint provided

- **GIVEN** a RUM JavaScript tag and no hint
- **WHEN** the manual instructions are rendered
- **THEN** the output contains no hint line
- **AND** all other lines are rendered unchanged
