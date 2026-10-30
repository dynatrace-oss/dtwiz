# Self-monitoring events

`dtwiz` reports what it does to your own Dynatrace environment as `CUSTOM_INFO` events. Use them to see which commands ran, how far each run got, which monitoring was set up, and how long it took until data arrived.

These pages describe every event, show an example of each, and list queries for common questions.

## Overview

- **Where events go:** `dtwiz` sends each event to the Events v2 API (`/api/v2/events/ingest`) of the environment in `DT_ENVIRONMENT` or `--environment`, authenticated with your platform token. Nothing is sent when no environment is configured.
- **What it costs:** every event counts against your environment's event ingest quota. A typical `dtwiz install` run sends between 3 and 12 events.
- **What is in them:** anonymous usage data only. Events never contain tokens, host names, user names, file paths, project or process names, cluster names, or command output. The only URL an event can contain is your own environment's URL (see [Errors](errors.md)).
- **Opt-out:** there is none. The events go only to your own environment.
- **Delivery:** events are sent in the background with a 3-second timeout and never slow down or fail a command. An event that can't be delivered is dropped. When `dtwiz` exits, it waits at most 500 ms for events still in flight.
- **Ordering:** events are sent independently, so two events sent at almost the same time can arrive in either order. Their `timestamp` is the time Dynatrace received them. To reconstruct a run, group by `executionId` instead of relying on order.
- **Rejected events:** when the token is missing, invalid, or lacks permission, the environment rejects the event and it never appears in `fetch events`. Failed credential checks therefore leave no trace in your environment.

In Grail, each event has `event.type == "CUSTOM_INFO"`, an `event.name` that starts with `dtwiz`, and the properties described below as top-level fields. All property values are strings; convert numbers with `toLong()` before calculating with them.

## Common properties

Every event of the `dtwiz` binary carries these properties:

| Property      | Example                      | Description                                                                                                                                              |
| ------------- | ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `event.name`  | `dtwiz install otel`         | `dtwiz`, followed by the command and subcommand when set                                                                                                 |
| `executionId` | `3ab`                        | Random 3-character hex ID, new for every run. Links the events of one run                                                                                |
| `step`        | `invoked`                    | What the event reports. See the step table below                                                                                                         |
| `version`     | `1.14.0`                     | `dtwiz` version that sent the event                                                                                                                      |
| `os`          | `darwin`, `linux`, `windows` | Operating system                                                                                                                                         |
| `mode`        | `tty`, `non-tty`, `debug`    | `tty` in an interactive terminal, `non-tty` when output is redirected or piped (for example in CI or when an agent runs `dtwiz`), `debug` with `--debug` |
| `command`     | `install`                    | Top-level command                                                                                                                                        |
| `subcommand`  | `otel`                       | Subcommand or method, when there is one                                                                                                                  |
| `error`       | `dependency_missing`         | Why the run failed or was cancelled. Only present when there is an error (see [Errors](errors.md))                                                       |

Steps:

| `step`                      | Sent when                                                                     |
| --------------------------- | ----------------------------------------------------------------------------- |
| `invoked`                   | A command starts                                                              |
| `analyze`                   | `setup` finished analyzing the system                                         |
| `recommendations_presented` | `setup` showed a recommendation in its menu, one event per entry              |
| `recommendations_selected`  | The user picked an entry in the `setup` menu                                  |
| `install`                   | An installer did install work. Reports the feature outcomes and the work time |
| `snapshot`                  | A watch session saw data of a new signal type for the first time              |
| `completed`                 | A command or watch session ended successfully                                 |
| `failed`                    | A command failed                                                              |
| `cancelled`                 | The user declined a confirmation or cancelled the `setup` menu                |

Every run that is not interrupted ends with exactly one `completed`, `failed`, or `cancelled` event for its command. A run whose `invoked` event has no such terminal event was killed, crashed, or lost its connection.

The examples on the following pages omit `event.name`, which equals the event title, and show the event title and properties as `dtwiz` sends them.

## Pages

| Page                                  | Contents                                                           |
| ------------------------------------- | ------------------------------------------------------------------ |
| [Install scripts](install-scripts.md) | Events of `install.sh` and `install.ps1`                           |
| [setup](setup.md)                     | The `dtwiz setup` wizard, from analysis to install                 |
| [install](install.md)                 | `dtwiz install <method>`, including feature outcomes and work time |
| [update](update.md)                   | `dtwiz update <method>`                                            |
| [uninstall](uninstall.md)             | `dtwiz uninstall <method>`                                         |
| [watch](watch.md)                     | Watch sessions and time to first data                              |
| [Other commands](other-commands.md)   | Help, `analyze`, `recommend`, `status`, `version`                  |
| [Errors](errors.md)                   | Error categories and their extra properties                        |
| [Request headers](request-headers.md) | The compact copy of each event in the HTTP headers                 |
| [Sample queries](queries.md)          | DQL queries for common questions                                   |
| [Older versions](older-versions.md)   | How install events looked up to 1.13.0                             |
