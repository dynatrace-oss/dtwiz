# `update` events

Each event also carries the [common properties](README.md#common-properties).

`dtwiz update <method>` covers `azure`, `gcp`, and the experimental `otel`.

```text
invoked ──▶ completed | failed | cancelled
```

`update azure` and `update gcp` run a watch session as part of the update, so their [watch](watch.md) events arrive before the terminal event. `update` sends no `install` event.

When the OTel Collector config is already up to date, `update otel` ends with `cancelled` and `error: "install_failed"`.

`invoked`:

```json
{
  "title": "dtwiz update azure",
  "properties": {
    "executionId": "2d8",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "update",
    "subcommand": "azure"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz update azure",
  "properties": {
    "executionId": "2d8",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "update",
    "subcommand": "azure"
  }
}
```

`failed`:

```json
{
  "title": "dtwiz update gcp",
  "properties": {
    "executionId": "2d9",
    "step": "failed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "update",
    "subcommand": "gcp",
    "error": "dependency_missing",
    "dependency.name": "gcloud"
  }
}
```

`cancelled`:

```json
{
  "title": "dtwiz update azure",
  "properties": {
    "executionId": "2da",
    "step": "cancelled",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "update",
    "subcommand": "azure",
    "error": "user_cancelled"
  }
}
```
