# `install` events

Each event also carries the [common properties](README.md#common-properties).

`dtwiz install <method>` covers `oneagent`, `kubernetes`, `otel`, `otel-collector`, `otel-python`, `otel-node`, `otel-java`, `aws`, `aws-lambda`, `azure`, `gcp`, `demo`, and the experimental `docker`.

```text
invoked ──▶ [install] ──▶ completed | failed | cancelled ──▶ watch events (command=watch)
```

| Outcome                                                                | Events                                                   |
| ---------------------------------------------------------------------- | -------------------------------------------------------- |
| Success                                                                | `invoked` ──▶ `install` ──▶ `completed` ──▶ watch events |
| Failure after install work started                                     | `invoked` ──▶ `install` ──▶ `failed`                     |
| Failure before install work started (missing tool, credentials, token) | `invoked` ──▶ `failed`                                   |
| Declined confirmation                                                  | `invoked` ──▶ `cancelled`                                |
| `--dry-run`                                                            | `invoked` ──▶ `completed`                                |

For `aws`, `azure`, and `gcp`, the installer runs the watch itself, so the watch events arrive before `install` and `completed`.

The `install` event reports what the install did. It never carries an error; the terminal event does. Its properties:

| Property                       | Values                             | Description                                                                                                  |
| ------------------------------ | ---------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `install.otel_config_written`  | `succeeded`, `failed`, `not_tried` | OTel Collector config written (`otel`, `otel-collector`, `demo`)                                             |
| `install.host_monitoring`      | `succeeded`, `failed`, `not_tried` | Host Monitoring enabled (`otel`, `otel-collector`, `demo`, `oneagent`, `docker`)                             |
| `install.rum`                  | `not_tried`                        | Reserved for Real User Monitoring                                                                            |
| `install.synthetic_monitoring` | `not_tried`                        | Reserved for Synthetic Monitoring                                                                            |
| `install.rds_extensions`       | `not_tried`                        | Reserved for the RDS extensions                                                                              |
| `install.duration_ms`          | `18450`                            | Time spent on install work, in milliseconds. Excludes the time spent answering prompts and the watch session |

`succeeded` means the feature is on after the run, including when it already was. `failed` means `dtwiz` tried and the feature is off; the install as a whole can still succeed. `not_tried` means the method doesn't support the feature, its conditions weren't met, or the install failed before reaching it.

`invoked`:

```json
{
  "title": "dtwiz install otel-collector",
  "properties": {
    "executionId": "c19",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "install",
    "subcommand": "otel-collector"
  }
}
```

`install`:

```json
{
  "title": "dtwiz install otel-collector",
  "properties": {
    "executionId": "c19",
    "step": "install",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "install",
    "subcommand": "otel-collector",
    "install.otel_config_written": "succeeded",
    "install.host_monitoring": "failed",
    "install.rum": "not_tried",
    "install.synthetic_monitoring": "not_tried",
    "install.rds_extensions": "not_tried",
    "install.duration_ms": "18450"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz install otel-collector",
  "properties": {
    "executionId": "c19",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "install",
    "subcommand": "otel-collector"
  }
}
```

`failed`:

```json
{
  "title": "dtwiz install otel-node",
  "properties": {
    "executionId": "c20",
    "step": "failed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "darwin",
    "command": "install",
    "subcommand": "otel-node",
    "error": "dependency_missing",
    "dependency.name": "npm"
  }
}
```

`cancelled`:

```json
{
  "title": "dtwiz install kubernetes",
  "properties": {
    "executionId": "c21",
    "step": "cancelled",
    "version": "1.14.0",
    "mode": "tty",
    "os": "darwin",
    "command": "install",
    "subcommand": "kubernetes",
    "error": "user_cancelled"
  }
}
```
