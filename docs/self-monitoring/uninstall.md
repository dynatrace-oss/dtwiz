# `uninstall` events

Each event also carries the [common properties](README.md#common-properties).

`dtwiz uninstall <method>` covers `oneagent`, `kubernetes`, `otel`, `aws`, `aws-lambda`, `azure`, `gcp`, and `self`.

```text
invoked ──▶ completed | failed | cancelled
```

`--dry-run` ends with `completed`.

`invoked`:

```json
{
  "title": "dtwiz uninstall kubernetes",
  "properties": {
    "executionId": "91b",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "uninstall",
    "subcommand": "kubernetes"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz uninstall kubernetes",
  "properties": {
    "executionId": "91b",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "uninstall",
    "subcommand": "kubernetes"
  }
}
```

`failed`:

```json
{
  "title": "dtwiz uninstall oneagent",
  "properties": {
    "executionId": "91c",
    "step": "failed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "uninstall",
    "subcommand": "oneagent",
    "error": "install_failed"
  }
}
```

`cancelled`:

```json
{
  "title": "dtwiz uninstall oneagent",
  "properties": {
    "executionId": "91d",
    "step": "cancelled",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "uninstall",
    "subcommand": "oneagent",
    "error": "user_cancelled"
  }
}
```
