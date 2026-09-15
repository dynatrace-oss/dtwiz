# Spec: RUM Frontend

## ADDED Requirements

### Requirement: Ensure a RUM frontend application exists for the project

The system SHALL provide an idempotent operation that guarantees a `WEB_AGENTLESS` Dynatrace frontend application exists for the current project and environment, returning its ID. If a frontend application ID is already stored in the project config for the given environment, the system SHALL return it without calling the Dynatrace API. If no ID is stored, the system SHALL create a new frontend application on the tenant, save the returned ID to the project config, and return it.

#### Scenario: Frontend application ID already stored

- **GIVEN** a project config that already contains a frontend application ID for the current environment URL
- **WHEN** the system is asked to ensure a frontend application exists
- **THEN** the system returns that ID without making any API call

#### Scenario: No frontend application ID stored

- **GIVEN** a project config with no frontend application ID for the current environment URL
- **WHEN** the system is asked to ensure a frontend application exists
- **THEN** the system creates a new frontend application on the Dynatrace tenant, saves its ID to the project config, and returns the ID

#### Scenario: API call fails

- **GIVEN** a project config with no frontend application ID for the current environment URL
- **WHEN** the system attempts to create a frontend application and the Dynatrace API returns an error
- **THEN** the system returns an error and does not modify the project config

### Requirement: Frontend name derived automatically from project directory

The system SHALL generate the frontend's unique name deterministically from the project directory path without requiring user input. The name SHALL be prefixed with `dtwiz-` to indicate its origin, and SHALL include a disambiguating suffix derived from the absolute project path to prevent collisions between projects that share a directory name.

#### Scenario: Name generated for a given project directory

- **GIVEN** a project directory
- **WHEN** the system generates a frontend name
- **THEN** the name starts with `dtwiz-`, contains a sanitized form of the directory basename, and ends with an 8-character hash of the absolute path

#### Scenario: Same directory always produces the same name

- **GIVEN** a specific absolute project path
- **WHEN** the system generates a frontend name for that path on two separate invocations
- **THEN** both invocations produce an identical frontend name

#### Scenario: Different directories with the same basename produce different names

- **GIVEN** two different absolute paths that share the same directory basename
- **WHEN** the system generates frontend names for each
- **THEN** the two names are not equal

### Requirement: Frontend display name labels dtwiz origin

The system SHALL set the frontend's display name to `{directory-basename} [dtwiz]` so that frontends created by dtwiz are identifiable in the Dynatrace UI.

#### Scenario: Display name format

- **GIVEN** a project directory named `my-app`
- **WHEN** a frontend is created for that project
- **THEN** the frontend's display name is `my-app [dtwiz]`
