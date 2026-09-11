# Spec: Self-Monitoring Watch Workflow

## NEW Requirements

### Requirement: dtwiz watch emits two self-monitoring events per invocation

The system SHALL emit two self-monitoring events per `dtwiz watch` invocation: one on invocation (`st=inv`) and one when first data is received or the session times out (`st=com`).

The `st=inv` event is sent by the standard root `PersistentPreRun` hook — `watch` does not override it. The `st=com` event is sent by a callback registered with `WatchIngestWithEvent` and fires mid-session (before the user exits).

#### Scenario: Command is invoked

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the user runs `dtwiz watch`
- **THEN** the system sends one `st=inv` event immediately via the standard root `PersistentPreRun` hook, with User-Agent `dtwiz/<ver>;c=wch;st=inv`

#### Scenario: First data is received during watch

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the first poll cycle after `dtwiz watch` starts returns at least one signal with data
- **THEN** the system sends one `st=com` event asynchronously (without waiting for the user to exit) with User-Agent `dtwiz/<ver>;st=com;t=<signals>` where `<signals>` is a positional comma-separated list of whole-second values (see "Signal timing encoding" below); the `c=` field is absent; the watch display continues uninterrupted

#### Scenario: Watch session ends with no data — user exits

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the user runs `dtwiz watch` and presses Enter before any signal receives data
- **THEN** only the `st=inv` event is present — no `st=com` event is sent; absence of the completion span indicates the user exited before data arrived

#### Scenario: Watch session times out with no data

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured
- **WHEN** the watch session reaches the 10-minute timeout and no signal data was seen
- **THEN** the system sends one `st=com` event with User-Agent `dtwiz/<ver>;st=com` — the `t=` field is absent because no signals were seen

#### Scenario: Watch session times out after data was already received

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and credentials are configured and first data was already received (`st=com` already fired)
- **WHEN** the watch session subsequently reaches the 10-minute timeout
- **THEN** no second completion event is sent — the `st=com` span was already emitted at first-data time

#### Scenario: Feature flag disabled

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is not enabled
- **WHEN** the user runs `dtwiz watch` and exits
- **THEN** no self-monitoring event is sent

### Requirement: Signal timing is encoded in the User-Agent header using positional CSV

The `t=` field in the User-Agent of the `st=com` event encodes time-to-first-data per signal type. The format is a positional comma-separated list of whole-second values, one per signal, in a fixed order. A signal that received no data is encoded as `0`; a signal that received data encodes as the number of whole seconds to first data, with a minimum of `1` (sub-second arrivals are encoded as `1`, not `0`). The `t=` field is omitted entirely when no signals received data.

**Fixed signal order (alphabetical):** `cld`, `exc`, `hst`, `k8s`, `log`, `rel`, `req`, `svc`

**Encoding invariant:** `0` always means the signal was not seen. A signal that was seen encodes as the whole-second duration (minimum `1`, even if the actual time was under 1 second). This preserves the ability to distinguish "absent" from "present but very fast".

**Examples:**

| Signals seen | Encoded value |
|---|---|
| `hst` after 12s, `k8s` after 5s, `rel` after 9s, `svc` after 3s | `t=0,0,12,5,0,9,0,3` |
| `hst` after 800ms (sub-second), `svc` after 400ms (sub-second) | `t=0,0,1,0,0,0,0,1` |
| All 8 signals | `t=60,45,12,5,30,9,25,3` |
| No signals | `t=` field absent |

**Worst-case header length:** `dtwiz/1.8.0;st=com;t=60,45,12,5,30,9,25,3` = 43 characters, well within the 64-character HAProxy capture limit.

#### Scenario: Partial signal data

- **GIVEN** a watch session where only some signals received data
- **WHEN** the `st=com` event is assembled
- **THEN** the `t=` value has a `0` for each absent signal and a whole-second value for each present signal, in fixed positional order

#### Scenario: All signals present

- **GIVEN** a watch session where all 8 signals received data
- **WHEN** the `st=com` event is assembled
- **THEN** the `t=` value has 8 comma-separated whole-second values in fixed order

## MODIFIED Requirements

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

### Requirement: st=com for watch omits c= from the User-Agent

For the `st=com` event sent by `dtwiz watch`, the `c=` field (command identifier) SHALL be absent from the User-Agent header. This distinguishes it from the `st=inv` event (which carries `c=wch`) and keeps the header short.

#### Scenario: Completion event header format

- **GIVEN** `DTWIZ_SELF_MONITORING_POC` is enabled and a watch session ends with data
- **WHEN** the `st=com` event is sent
- **THEN** the User-Agent is `dtwiz/<ver>;st=com;t=<signals>` — no `c=wch` segment is present
