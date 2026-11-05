# `watch` events

Each event also carries the [common properties](README.md#common-properties).

A watch session polls your environment for new data and shows a live summary. It runs either as `dtwiz watch` or automatically after `install`, `setup`, `update azure`, and `update gcp`.

```text
dtwiz watch:   invoked ──▶ snapshot ×0..8 ──▶ completed
post-install:  snapshot ×0..8 ──▶ completed            (no invoked)
```

Watch events always have `command` set to `watch` and no `subcommand`, also when an install started the session. To find the command that started a post-install session, look up the other events with the same `executionId`.

- **`snapshot`** is sent each time a signal type receives data for the first time. It carries the time to first data of every signal type seen so far, so the latest `snapshot` of a session has the complete picture. No `snapshot` is sent when no data arrives.
- **`completed`** is sent when the session ends, either because the user exits or because it times out. It carries no timings. When the process is killed, there is no `completed`, but the `snapshot` events sent until then remain valid.

When `dtwiz watch` fails its own credential check, it exits after `invoked` without a terminal event.

`snapshot` properties, one per signal type that received data, each in milliseconds since the session started:

| Property        | Signal                         |
| --------------- | ------------------------------ |
| `hosts`         | Hosts                          |
| `services`      | Services                       |
| `requests`      | Requests                       |
| `exceptions`    | Exceptions                     |
| `logs`          | Logs                           |
| `kubernetes`    | Kubernetes entities            |
| `cloud`         | Cloud resources                |
| `relationships` | Relationships between entities |

`snapshot` also carries a `type` property, a compact encoding of the same timings used for the request headers. Use the named properties instead.

`snapshot`:

```json
{
  "title": "dtwiz watch",
  "properties": {
    "executionId": "c19",
    "step": "snapshot",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "watch",
    "type": "0,0,12,0,5,0,0,0",
    "hosts": "12030",
    "logs": "5310"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz watch",
  "properties": {
    "executionId": "c19",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "watch"
  }
}
```
