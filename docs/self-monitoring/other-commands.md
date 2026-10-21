# Help, analyze, recommend, status, and version events

Each event also carries the [common properties](README.md#common-properties).

## Help

`dtwiz --help` and `--help` on any command send a single `invoked` event with `command` set to `help` and `subcommand` set to the command whose help was shown, if it isn't the root command. There is no terminal event.

```text
invoked
```

`dtwiz` without arguments sends two `invoked` events: one with `command: "dtwiz"` (title `dtwiz dtwiz`), and one with `command: "help"` and no `subcommand`.

`dtwiz install otel --help`:

```json
{
  "title": "dtwiz help otel",
  "properties": {
    "executionId": "a07",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "help",
    "subcommand": "otel"
  }
}
```

## analyze, recommend, status, version

```text
invoked ──▶ completed
```

These commands always end with `completed`. When `analyze`, `recommend`, or `status` fail, the `completed` event carries `error: "err"`, without a more specific category.

`invoked`:

```json
{
  "title": "dtwiz status",
  "properties": {
    "executionId": "e41",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "darwin",
    "command": "status"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz status",
  "properties": {
    "executionId": "e41",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "darwin",
    "command": "status"
  }
}
```

`completed` after a failure:

```json
{
  "title": "dtwiz analyze",
  "properties": {
    "executionId": "e42",
    "step": "completed",
    "version": "1.14.0",
    "mode": "non-tty",
    "os": "linux",
    "command": "analyze",
    "error": "err"
  }
}
```
