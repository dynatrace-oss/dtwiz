# `setup` events

Each event also carries the [common properties](README.md#common-properties).

```text
invoked ──▶ analyze ──▶ recommendations_presented ×N ──▶ recommendations_selected
        ──▶ [install] ──▶ completed | failed | cancelled ──▶ watch events (command=watch)
```

1. `invoked` when `setup` starts.
2. `analyze` when the system analysis finished. It carries the detected cloud providers and Kubernetes distribution. When the analysis fails, it carries `error: "err"` and is followed by `failed`.
3. `recommendations_presented`, one per entry in the menu, with the method in `option`. When there is nothing to recommend, the run ends here without a terminal event.
4. `recommendations_selected` with the picked `option`. When the user cancels the menu (`0` or Enter), `cancelled` follows instead. When the user picks the uninstall help (`u`), the run ends here.
5. `install` when the installer did install work. Its `subcommand` is the selected method.
6. Exactly one terminal event for the `setup` command. It carries the install error, if any, and has no `subcommand`.
7. When the install succeeded and was not a dry-run, the [watch](watch.md) events follow. For `aws`, `azure`, and `gcp`, the installer runs the watch itself, so the watch events arrive before `install` and the terminal event.

| Outcome of the selected install                                     | Events after `recommendations_selected` |
| ------------------------------------------------------------------- | --------------------------------------- |
| Success                                                             | `install` ──▶ `completed`               |
| Failure after install work started                                  | `install` ──▶ `failed`                  |
| Failure before install work started (missing tool, bad credentials) | `failed`                                |
| Declined confirmation                                               | `cancelled`                             |
| OTel Collector config already up to date                            | `cancelled`                             |
| `--dry-run`                                                         | `completed`                             |

Values of `option`: `oneagent`, `kubernetes`, `docker`, `otel`, `otel-update`, `aws`, `azure`, `azure-update`, `gcp`, `gcp-update`, plus `demo` and `uninstall` for `recommendations_selected`.

`invoked`:

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "invoked",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup"
  }
}
```

`analyze`:

| Property         | Example   | Description                                                                                                                                                                                                 |
| ---------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloud.provider` | `aws,gcp` | Detected cloud providers, comma-separated: `aws`, `azure`, `gcp`. Absent when none                                                                                                                          |
| `k8s.distro`     | `EKS`     | Detected Kubernetes distribution: `GKE`, `GKE-Autopilot`, `EKS`, `EKS-Bottlerocket`, `AKS`, `IKS`, `OpenShift`, `k3s`, `RKE`, `minikube`, `kind`, `TKGI`, `kubernetes`. Absent when no cluster is reachable |

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "analyze",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "cloud.provider": "aws",
    "k8s.distro": "EKS"
  }
}
```

`recommendations_presented` (one of several):

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "recommendations_presented",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "option": "kubernetes"
  }
}
```

`recommendations_selected`:

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "recommendations_selected",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "option": "otel"
  }
}
```

`install`: same properties as the [install event](install.md) of `dtwiz install`, with `command` set to `setup`:

```json
{
  "title": "dtwiz setup otel",
  "properties": {
    "executionId": "7f2",
    "step": "install",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "subcommand": "otel",
    "install.otel_config_written": "succeeded",
    "install.host_monitoring": "succeeded",
    "install.rum": "not_tried",
    "install.synthetic_monitoring": "not_tried",
    "install.rds_extensions": "not_tried",
    "install.duration_ms": "24310"
  }
}
```

`completed`:

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "completed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup"
  }
}
```

`failed`:

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "failed",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "error": "dependency_missing",
    "dependency.name": "java"
  }
}
```

`cancelled`:

```json
{
  "title": "dtwiz setup",
  "properties": {
    "executionId": "7f2",
    "step": "cancelled",
    "version": "1.14.0",
    "mode": "tty",
    "os": "linux",
    "command": "setup",
    "error": "user_cancelled"
  }
}
```

When the OTel Collector config is already up to date, the `cancelled` event currently carries `error: "install_failed"` instead of `user_cancelled`.
