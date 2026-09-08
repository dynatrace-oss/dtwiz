# Spec: Self-Monitoring Events

## NEW Requirements

### Requirement: Self-monitoring events capture command invocations

The system SHALL send a `CUSTOM_INFO` event to the Dynatrace Events v2 API on every dtwiz command invocation when the self-monitoring feature is enabled, capturing per-invocation metadata in both the event body and HTTP headers.

#### Scenario: Event sent on command invocation with feature flag enabled

- **WHEN** `DTWIZ_SELF_MONITORING_POC=true` and user runs any dtwiz command
- **THEN** the system sends a `CUSTOM_INFO` event to `/api/v2/events/ingest` within 3 seconds
- **AND** the event contains metadata in the request body and headers

#### Scenario: No event sent when feature flag disabled

- **WHEN** `DTWIZ_SELF_MONITORING_POC` is not set or false
- **THEN** the system does not send any self-monitoring events

#### Scenario: Event silently fails if credentials not configured

- **WHEN** user runs a command with the feature enabled but no Dynatrace credentials configured
- **THEN** the system does not send an event
- **AND** the command continues executing normally
- **AND** no error is shown to the user

#### Scenario: Asynchronous event send does not block command execution

- **WHEN** a command is invoked with the feature enabled
- **THEN** event parameters are captured synchronously in `PersistentPreRun`
- **AND** the HTTP send operation runs in a background goroutine
- **AND** the command continues executing immediately without waiting for the event to be sent

### Requirement: Event body contains command and execution metadata

The event body JSON SHALL contain properties with always-present keys (`e`, `c`, `st`) and optional keys (`s`, `er`, `t`) when non-empty.

#### Scenario: Event body with command and step

- **WHEN** user runs `dtwiz install otel`
- **THEN** the event body contains properties:
  - `e`: 2-char hex execution ID (e.g., `"a3"`)
  - `c`: abbreviated command name (e.g., `"ins"`)
  - `st`: step identifier (e.g., `"inv"`)

#### Scenario: Event body with subcommand

- **WHEN** user runs a command with a subcommand like `dtwiz install otel`
- **THEN** the event body includes property `s` with the abbreviated subcommand name (e.g., `"otel"`)

#### Scenario: Event body without subcommand

- **WHEN** user runs a command without a subcommand like `dtwiz analyze`
- **THEN** the event body does not include property `s`

#### Scenario: Event body with optional fields

- **WHEN** a command fails or produces an error, or explicitly sets an event type
- **THEN** optional properties `er` (error) and `t` (type) are included only when non-empty
- **AND** all other command invocations omit these properties

### Requirement: HTTP headers encode execution context

The event request SHALL include two custom headers that encode execution context for HAProxy capture and pipeline routing.

#### Scenario: User-Agent header format

- **WHEN** an event is sent
- **THEN** the `User-Agent` header contains format `dtwiz/<version>;c=<cmd>;st=<step>[;s=<sub>][;er=<err>][;t=<type>]`
- **AND** the header is at most 64 characters
- **AND** optional segments are omitted when empty

#### Scenario: Tab-Id header format

- **WHEN** an event is sent
- **THEN** the `Tab-Id` header contains format `<execid>;m=<mode>;o=<os>`
- **AND** the header is at most 16 characters
- **AND** `<execid>` is 2 hex characters (e.g., `"a3"`)
- **AND** `<mode>` is 3 characters: `"deb"` (debug), `"tty"` (terminal), or `"ntt"` (non-terminal)
- **AND** `<os>` is 3 characters: `"mac"` (darwin), `"lin"` (linux), or `"win"` (windows)

#### Scenario: Mode detection

- **WHEN** the command is run with `-v` or `--debug` flag set
- **THEN** the mode is `"deb"`

- **WHEN** the command is run from an interactive terminal without debug flag
- **THEN** the mode is `"tty"`

- **WHEN** the command is run from a non-terminal context (pipe, CI, redirect)
- **THEN** the mode is `"ntt"`

#### Scenario: Custom monitoring header

- **WHEN** an event is sent
- **THEN** the request includes header `dtwiz-monitoring: dtwiz-start`
- **AND** this header is used for pipeline routing and header-based queries

### Requirement: Execution ID is stable within a process

The execution ID SHALL be generated once per process and reused for all events sent within that invocation.

#### Scenario: Consistent execution ID across multiple events

- **WHEN** a single dtwiz process sends multiple events (in future stages)
- **THEN** all events share the same execution ID
- **AND** the execution ID is a 2-character hex string (256 possible values)

#### Scenario: Execution ID generation on startup

- **WHEN** dtwiz starts
- **THEN** the system generates a random execution ID from `crypto/rand`
- **AND** if generation fails, the system falls back to `"00"`

### Requirement: Command and subcommand names are abbreviated

Command and subcommand names SHALL be abbreviated to fixed-length codes to keep headers compact.

#### Scenario: Command abbreviations

- **WHEN** a command is invoked
- **THEN** it is encoded as: `install`→`ins`, `uninstall`→`uni`, `update`→`upd`, `analyze`→`ana`, `recommend`→`rec`, `status`→`sta`, `watch`→`wch`, `setup`→`set`, `version`→`ver`

#### Scenario: Subcommand abbreviations

- **WHEN** a command has a subcommand
- **THEN** it is encoded as: `otel`→`otel`, `otel-collector`→`otlc`, `otel-python`→`otlp`, `otel-node`→`otln`, `otel-java`→`otlj`, `kubernetes`→`k8s`, `oneagent`→`oa`, `gcp`→`gcp`, `azure`→`az`, `aws`→`aws`, `aws-lambda`→`awsl`, `docker`→`dock`, `demo`→`demo`, `self`→`self`

#### Scenario: Unknown command or subcommand passes through unchanged

- **WHEN** a new command or subcommand is added that is not in the abbreviation map
- **THEN** the full name is used in the event
- **AND** the name is truncated as needed to fit header size constraints

### Requirement: Event sending does not surface errors to users

Event send failures and timeouts SHALL be logged internally but never returned to the caller or displayed to the user.

#### Scenario: Network error during event send

- **WHEN** the system encounters a network error while sending the event (timeout, connection refused, etc.)
- **THEN** the error is logged at debug level
- **AND** the command completes normally as if the event was not sent
- **AND** no error message appears to the user

#### Scenario: Invalid credentials fail silently

- **WHEN** the system has invalid or missing Dynatrace credentials
- **THEN** the event send fails silently without blocking the command
- **AND** no diagnostic message is shown to the user (error is logged at debug level only)

#### Scenario: HTTP send timeout

- **WHEN** the Dynatrace Events API does not respond within 3 seconds
- **THEN** the HTTP client times out the request
- **AND** the timeout is swallowed and does not interrupt the command
