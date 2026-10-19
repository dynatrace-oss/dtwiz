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

### Requirement: Name conflict on creation is reported with recovery guidance

When creating a frontend application fails because a frontend with the same name already exists on the tenant, the system SHALL report this as a distinct, actionable error instead of a generic failure. The error SHALL state the name of the conflicting frontend, point the user directly to the list of frontends on the tenant, and describe the two ways to resolve it: reuse the existing frontend by copying its ID into the project config, or delete the existing frontend and run again. The system SHALL NOT try to find or adopt the existing frontend on its own, and SHALL NOT modify the project config.

This situation arises when an earlier run created the frontend but could not save its ID, or when the project config was deleted.

#### Scenario: Frontend with the same name already exists

- **GIVEN** a project config with no frontend application ID for the current environment URL, and a frontend with the generated name already on the tenant
- **WHEN** the system tries to create the frontend application and the tenant rejects it because the name is not unique
- **THEN** the system returns an error that names the frontend, links to the tenant's frontend list, and explains both recovery options
- **AND** the project config is not modified

#### Scenario: Other creation failures are not reported as name conflicts

- **GIVEN** a project config with no frontend application ID for the current environment URL
- **WHEN** the tenant rejects the creation for any reason other than a name conflict
- **THEN** the system returns a generic creation error that includes the tenant's response

### Requirement: Missing permission is reported with the required scope

When the tenant rejects frontend creation because the credentials are not authenticated or not permitted, the system SHALL return an error that names the platform token scope needed to create frontends and tells the user to add it to their token. The message SHALL present this as the most likely cause, not a certain diagnosis, because the tenant's response cannot distinguish a missing scope from a policy that restricts it. The project config SHALL NOT be modified.

#### Scenario: Token lacks permission to create frontends

- **GIVEN** a project config with no frontend application ID for the current environment URL
- **WHEN** the tenant rejects the creation as unauthenticated or forbidden
- **THEN** the system returns an error that names the required scope, suggests adding it to the platform token, and includes the tenant's response
- **AND** the project config is not modified

### Requirement: Frontend provisioning failure is non-fatal and reported

Because RUM is an additional feature, a failure to provision the frontend application SHALL NOT cause the surrounding install to fail. The system SHALL surface every provisioning failure, including API errors, name conflicts and project config read or write errors, as an error to its caller. When provisioning fails, the install SHALL:

- warn the user with the error,
- skip all remaining RUM setup steps,
- report the RUM feature as failed in the install self-monitoring data,
- continue with the rest of the install.

The RUM feature SHALL be reported as succeeded only after the entire RUM setup has completed, not merely after the frontend application exists.

#### Scenario: Frontend creation fails during install

- **GIVEN** an install that includes RUM setup
- **WHEN** provisioning the frontend application fails
- **THEN** the user is warned, no further RUM steps run, the RUM feature is reported as failed, and the rest of the install completes normally

#### Scenario: Frontend exists but a later RUM step fails

- **GIVEN** the frontend application was provisioned successfully
- **WHEN** a later RUM step fails
- **THEN** the RUM feature is reported as failed, not succeeded

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
