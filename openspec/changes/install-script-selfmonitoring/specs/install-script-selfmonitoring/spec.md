# Spec: Install Script Self-Monitoring

## ADDED Requirements

### Requirement: Install scripts report their lifecycle

Both install scripts (bash `install.sh` and PowerShell `install.ps1`) SHALL send a `CUSTOM_INFO` self-monitoring event for each lifecycle point of an install run: invoked, then exactly one outcome of completed, failed, or cancelled. Step codes and body names reuse the binary's vocabulary: `inv`/`invoked`, `com`/`completed`, `fai`/`failed`, `can`/`cancelled`.

#### Scenario: Invoked event is sent first

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the user starts an install script
- **THEN** an `invoked` event is sent before argument parsing, the supported-platform checks, or any network access to GitHub

#### Scenario: Successful install sends completed

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the dtwiz binary has been placed in the install directory
- **THEN** a `completed` event is sent carrying the resolved version

#### Scenario: Declined confirmation sends cancelled

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the user answers no at the confirmation prompt
- **THEN** a `cancelled` event is sent with no error reason, and no failed or completed event follows

#### Scenario: Failure sends failed with a reason

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the script stops because of an error
- **THEN** exactly one `failed` event is sent carrying the failure reason, and no completed event follows

#### Scenario: Interrupted PowerShell run sends cancelled with interrupted

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the user presses Ctrl-C while the PowerShell script runs, before an outcome was sent
- **THEN** a `cancelled` event is sent with the reason `interrupted`, and no failed or completed event follows

#### Scenario: Killed run leaves only invoked

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the script is killed before it finishes (closing the window, or Ctrl-C in the bash script)
- **THEN** only the `invoked` event exists for that run

### Requirement: Events are sent only to a configured environment

The scripts SHALL send events to the Dynatrace environment configured for dtwiz (the same setting the binary reads), normalized to its classic URL form in the same way the binary does (platform `.apps.` segment removed, trailing slash removed). When a platform token is configured (the same setting the binary reads) it SHALL be used for authorization; when it is absent the event SHALL still be sent without authorization, because the request headers are captured at the environment's edge even when ingest rejects it.

#### Scenario: No environment configured

- **GIVEN** no Dynatrace environment is configured
- **WHEN** the user runs an install script
- **THEN** no event is sent and the install proceeds unchanged

#### Scenario: Platform URL is normalized

- **GIVEN** the configured environment URL is `https://abc123.apps.dynatrace.com/`
- **WHEN** an event is sent
- **THEN** it is sent to the classic environment `https://abc123.dynatrace.com`

#### Scenario: No token configured

- **GIVEN** a Dynatrace environment is configured and no platform token is configured
- **WHEN** an event is due
- **THEN** the event is sent without authorization

#### Scenario: Token is not exposed

- **GIVEN** a platform token is configured
- **WHEN** the bash script sends an event
- **THEN** the token does not appear in the command line of any process started by the script

### Requirement: Sending never disturbs the install

Event sending SHALL be silent and bounded: it SHALL NOT print output, SHALL NOT change the script's exit status or control flow, and SHALL give up after at most 3 seconds per event.

#### Scenario: Environment unreachable

- **GIVEN** the configured environment URL points to an unreachable host
- **WHEN** the user runs an install script
- **THEN** the install completes as it would without self-monitoring, each event attempt adds at most 3 seconds, and no error from sending is shown

#### Scenario: Ingest rejects the event

- **GIVEN** the environment answers an event with an error status
- **WHEN** the event is sent
- **THEN** nothing is printed and the script continues

### Requirement: User-Agent header encodes the event

Each event request SHALL carry a User-Agent of the form `dtwiz/install;s=<script>;st=<step>[;v=<version>][;er=<reason>]`, at most 64 characters, where `<script>` is `sh` or `ps1`. The `v=` field SHALL be present once the version has been resolved and absent before. The `er=` field SHALL be present only on failed events.

#### Scenario: Invoked event header

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the bash script sends its invoked event
- **THEN** the User-Agent is `dtwiz/install;s=sh;st=inv`

#### Scenario: Failure after version resolution

- **GIVEN** the PowerShell script resolved version `0.9.0`
- **WHEN** the download fails
- **THEN** the User-Agent is `dtwiz/install;s=ps1;st=fai;v=0.9.0;er=download_failed`

#### Scenario: Failure before version resolution

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the bash script fails because the architecture is unsupported
- **THEN** the User-Agent is `dtwiz/install;s=sh;st=fai;er=unsupported_arch` with no `v=` field

#### Scenario: Header length

- **GIVEN** a Dynatrace environment is configured
- **WHEN** any event is sent with a version of up to 12 characters
- **THEN** the User-Agent is at most 64 characters

### Requirement: Correlation and OS travel in the Tab-Id header

Each event request SHALL carry a Tab-Id header of the form `<execid>;o=<os>`, where `<execid>` is 3 random lowercase hex characters chosen once per script run and `<os>` is `lin`, `mac`, or `win`. For an operating system outside these, `<os>` SHALL be the first 3 characters of the lowercased system name. Each request SHALL also carry the header `dtwiz-monitoring: dtwiz-start` used to route the binary's events.

#### Scenario: Stable id within a run

- **GIVEN** a Dynatrace environment is configured
- **WHEN** a script run sends its invoked and outcome events
- **THEN** both carry the same execution id

#### Scenario: PowerShell on Linux

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the PowerShell script runs in PowerShell on Linux
- **THEN** the Tab-Id reports `o=lin`

#### Scenario: Windows PowerShell 5.1

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the PowerShell script runs in Windows PowerShell 5.1
- **THEN** the Tab-Id reports `o=win`

### Requirement: Event body carries the full values

Each event SHALL have the title `dtwiz install script` and the properties `executionId`, `script` (`sh` or `ps1`), `step` (`invoked`, `completed`, `failed`, `cancelled`), `os` (`linux`, `darwin`, `windows`, or the lowercased system name otherwise), and `channel` (`stable`, `main`, or `tag`). It SHALL add `tag` when a release tag was requested, `version` once resolved, and `error` on failed events and on interrupted cancelled events (`interrupted`).

#### Scenario: Snapshot branch install

- **GIVEN** the user requested release tag `snapshot-feature-x` and its version resolves to `0.9.1-next`
- **WHEN** the install completes
- **THEN** the completed event has `channel=tag`, `tag=snapshot-feature-x`, and `version=0.9.1-next`

#### Scenario: Main channel install

- **GIVEN** the user requested the `main` channel
- **WHEN** the invoked event is sent
- **THEN** it has `channel=main` and `tag=snapshot-main`

#### Scenario: Default install

- **GIVEN** the user requested neither a release tag nor a channel
- **WHEN** the invoked event is sent
- **THEN** it has `channel=stable` and no `tag` property

### Requirement: Versions match the binary's format

The reported version SHALL be the version of the release archive being installed, without a leading `v`, so it equals the version the installed binary reports about itself.

#### Scenario: Stable release tag

- **GIVEN** the latest stable release tag is `v0.9.0`
- **WHEN** the version is reported
- **THEN** it is `0.9.0`

### Requirement: Failure reasons

Every failure SHALL be reported with exactly one of these reasons:

| Reason              | Cause                                                                         |
| ------------------- | ----------------------------------------------------------------------------- |
| `unknown_argument`  | an unrecognized command-line argument (bash only)                             |
| `unsupported_os`    | an operating system the script does not support                               |
| `unsupported_arch`  | an architecture the script does not support                                   |
| `curl_missing`      | curl is not available (bash only)                                             |
| `version_not_found` | the requested or latest version cannot be determined                          |
| `install_dir_failed` | the default install directory cannot be determined (PowerShell only)         |
| `prompt_failed`     | the confirmation prompt cannot be shown, e.g. a non-interactive session (PowerShell only) |
| `download_failed`   | the release archive cannot be downloaded                                      |
| `extract_failed`    | the release archive cannot be extracted                                       |
| `binary_missing`    | the archive does not contain the dtwiz binary                                 |
| `move_failed`       | the install directory cannot be created or the binary cannot be moved into it |

#### Scenario: Elevated move fails

- **GIVEN** the install directory needs elevated privileges
- **WHEN** the privileged move of the binary fails (for example, the sudo password is refused)
- **THEN** the bash script reports `move_failed`, prints an error, and does not report the install as completed

#### Scenario: Download fails

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the release archive download fails
- **THEN** the script reports `download_failed`, not `extract_failed` or `binary_missing`

#### Scenario: Stable version lookup fails

- **GIVEN** no release tag was requested
- **WHEN** the latest release cannot be determined (no network, or the answer is not a version tag)
- **THEN** the script reports `version_not_found` and does not attempt a download

#### Scenario: PowerShell outside Windows

- **GIVEN** a Dynatrace environment is configured
- **WHEN** the PowerShell script runs on Linux or macOS
- **THEN** it reports `unsupported_os` and tells the user to use `install.sh` instead

#### Scenario: PowerShell runtime errors are reported

- **GIVEN** a Dynatrace environment is configured
- **WHEN** a PowerShell download, extraction, or move step throws an error
- **THEN** the script reports the matching reason before the error reaches the user
