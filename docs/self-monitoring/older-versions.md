# Events from older versions

Up to and including `dtwiz` 1.13.0, install events had a different shape:

- `dtwiz install <method>` sent no `completed`, `failed`, or `cancelled` event after the installer ran. The `install` event was the last event of the run.
- The `install` event carried `error` and the error properties. In `dtwiz install`, it was also sent for dry-runs and declined confirmations, without feature outcomes.
- In `setup`, a failed install reported its error on both `install` and `failed`.

To include older versions in failure queries, also read `error` from `install` events of those versions:

```dql
fetch events, from: now() - 90d
| filter event.type == "CUSTOM_INFO" and command == "install"
| filter step == "failed" or (step == "install" and isNotNull(error))
| summarize failures = count(), by: { version, subcommand, error }
```
