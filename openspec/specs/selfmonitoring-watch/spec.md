# Spec: Self-Monitoring Watch Workflow

## Purpose

Record how quickly each kind of data first appears in the tenant during a watch session, so the
time-to-first-data of different signal types can be compared across sessions and installs.

## Requirements

### Requirement: dtwiz watch emits two self-monitoring events per invocation

The system SHALL emit one `st=inv` event on invocation, one `st=snp` snapshot event each time the set of signal types with data grows, and one `st=com` completion event when the session ends. A session therefore produces between two and ten events.

Each snapshot SHALL carry the cumulative time-to-first-data for every signal type seen so far. A recorded value SHALL NOT change for the remainder of the session, so each snapshot repeats all earlier values and adds the new one. The completion event SHALL NOT carry signal timing data.

This applies to every watch session, whether started by `dtwiz watch` or by a post-install watch.

#### Scenario: Command is invoked

- **GIVEN** a watch session is starting
- **WHEN** the user runs `dtwiz watch`
- **THEN** the system sends one `st=inv` event immediately, with User-Agent `dtwiz/<ver>;c=wch;st=inv`

#### Scenario: First signal type receives data

- **GIVEN** a running watch session in which no signal type has data yet
- **WHEN** a poll cycle reports data for exactly one signal type
- **THEN** the system sends one `st=snp` event asynchronously, without waiting for the user to exit, with User-Agent `dtwiz/<ver>;c=wch;st=snp;t=<signals>` where `<signals>` is the positional comma-separated encoding described below; the `s=` subcommand field is absent; each seen signal type also appears as a named property in the event body in milliseconds (e.g. `hosts: "1200"`); the watch display continues uninterrupted

#### Scenario: A further signal type receives data later in the session

- **GIVEN** a running watch session that has already reported hosts at 1 second
- **WHEN** a later poll cycle reports data for services for the first time, 8 seconds into the session
- **THEN** the system sends a second `st=snp` event whose `t=` value carries both the unchanged hosts value and the new services value, and whose body properties contain both `hosts` and `services`

#### Scenario: Several signal types become visible in the same poll cycle

- **GIVEN** a running watch session
- **WHEN** a single poll cycle reports data for two signal types that were both previously unseen
- **THEN** the system sends exactly one `st=snp` event covering both, not one event per signal type

#### Scenario: A signal type that was already seen reports data again

- **GIVEN** a running watch session that has already reported hosts
- **WHEN** a later poll cycle reports hosts data again and no previously unseen signal type has data
- **THEN** no new `st=snp` event is sent, because the set of seen signal types did not grow

#### Scenario: Watch session ends after data was seen

- **GIVEN** a watch session that has sent at least one `st=snp` event
- **WHEN** the session ends, whether by the user pressing Enter, by standard input closing, or by the 10-minute timeout being declined or reached in a non-interactive session
- **THEN** the system sends one `st=com` event with User-Agent `dtwiz/<ver>;c=wch;st=com`, carrying neither a `t=` field nor signal body properties

#### Scenario: Watch session ends with no data

- **GIVEN** a watch session that has sent no `st=snp` event
- **WHEN** the session ends by any means
- **THEN** the system sends one `st=com` event with User-Agent `dtwiz/<ver>;c=wch;st=com`; the combination of `st=inv` and `st=com` with no intervening `st=snp` identifies a session in which no signal type ever received data

#### Scenario: Watch process ends without a clean exit

- **GIVEN** a watch session that has sent one or more `st=snp` events
- **WHEN** the process is terminated before the session can end normally
- **THEN** no `st=com` event is sent, and the already-sent `st=snp` events remain valid and independently interpretable, because each carries a complete cumulative snapshot rather than a delta

#### Scenario: Watch session is extended past the timeout prompt

- **GIVEN** a watch session in which the user was prompted at the 10-minute mark and chose to continue watching
- **WHEN** a signal type receives data for the first time after the continuation
- **THEN** its recorded time-to-first-data is measured from the start of the watch session, not from the continuation, so its value exceeds 600 seconds

#### Scenario: Watch session triggered by post-install watch

- **GIVEN** a `dtwiz install <method>` command that triggers a post-install watch
- **WHEN** the watch session emits its `st=snp` and `st=com` events
- **THEN** the User-Agent of those events carries `c=wch`, not the triggering install command, while the `st=inv` event for the invocation carries the install command as usual

### Requirement: Signal timing is encoded in the User-Agent header using positional CSV

The `t=` field of the `st=snp` event SHALL encode time-to-first-data per signal type as a positional comma-separated list of whole-second values, one per signal type, in a fixed order. A signal type with no data encodes as `0`; a signal type with data encodes as whole seconds from the start of the watch session, minimum `1`, clamped to a maximum of `999`. `0` therefore always means "not seen", keeping "absent" distinguishable from "present but very fast".

`t=` SHALL always be present on `st=snp` and never on `st=com`. The event body properties carry the unclamped value in milliseconds.

**Fixed signal order (alphabetical):** `cld`, `exc`, `hst`, `k8s`, `log`, `rel`, `req`, `svc`

#### Scenario: Partial signal data

- **GIVEN** a watch session where only some signal types have received data
- **WHEN** an `st=snp` event is assembled
- **THEN** the `t=` value has a `0` for each signal type not yet seen and a whole-second value for each signal type seen, in fixed positional order

#### Scenario: All signals present

- **GIVEN** a watch session where all 8 signal types have received data
- **WHEN** an `st=snp` event is assembled
- **THEN** the `t=` value has 8 comma-separated whole-second values in fixed order

#### Scenario: Signal first seen beyond the clamp threshold

- **GIVEN** a watch session extended past the timeout prompt, in which a signal type first receives data 1500 seconds after the session started
- **WHEN** the `st=snp` event is assembled
- **THEN** that signal type's position in the `t=` value is `999`, and its event body property carries the unclamped value `1500000`

#### Scenario: Header stays within the capture limit

- **GIVEN** a watch session in which all 8 signal types are first seen beyond the clamp threshold
- **WHEN** the `st=snp` event is assembled
- **THEN** the full User-Agent header is at most 64 characters long

### Requirement: Self-monitoring events carry a meaningful title

The system SHALL set the event title to `"dtwiz <cmd>"` or `"dtwiz <cmd> <sub>"` (using the normalized command and subcommand identifiers) for all self-monitoring events.

#### Scenario: Top-level command event

- **GIVEN** a self-monitoring event is sent for a top-level command (e.g. `watch`, `status`)
- **WHEN** the event is ingested
- **THEN** the event title is `"dtwiz <cmd>"` (e.g. `"dtwiz wch"`)

#### Scenario: Subcommand event

- **GIVEN** a self-monitoring event is sent for a subcommand (e.g. `install otel`)
- **WHEN** the event is ingested
- **THEN** the event title is `"dtwiz <cmd> <sub>"` (e.g. `"dtwiz ins otel"`)
