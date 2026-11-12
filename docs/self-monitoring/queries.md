# Sample queries

Run these in a Notebook or Dashboard in your environment. Adjust the timeframe as needed.

Because `executionId` has only 4096 possible values, two runs can share one over a long timeframe. Queries that group by `executionId` use a short timeframe and also group by `version` and `os` to keep collisions unlikely.

## Recent runs

```dql
fetch events, from: now() - 7d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| fields timestamp, executionId, event.name, step, error, version, os, mode
| sort timestamp desc
| limit 200
```

## Timeline of one run

```dql
fetch events, from: now() - 1d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter executionId == "c19"
| sort timestamp asc
```

## Outcome of runs per command

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter in(step, array("completed", "failed", "cancelled")) and command != "watch"
| summarize runs = count(), by: { command, subcommand, step }
| sort command, subcommand, step
```

## Most common failures

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter step == "failed"
| summarize failures = count(), by: { command, subcommand, error, dependency.name }
| sort failures desc
```

## Runs that never finished

Runs that were killed, crashed, or lost their connection before sending a terminal event:

```dql
fetch events, from: now() - 1d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter not in(command, array("watch", "help", "dtwiz"))
| summarize steps = collectDistinct(step), started = min(timestamp), by: { executionId, version, os, command, subcommand }
| filter in("invoked", steps)
| filter not (in("completed", steps) or in("failed", steps) or in("cancelled", steps))
| sort started desc
```

## Feature outcomes per install method

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter step == "install"
| summarize installs = count(), by: { subcommand, install.otel_config_written, install.host_monitoring }
| sort subcommand
```

## Install work time per method

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter step == "install"
| fieldsAdd duration_s = toDouble(install.duration_ms) / 1000
| summarize installs = count(), p50 = percentile(duration_s, 50), p90 = percentile(duration_s, 90), by: { subcommand }
| sort p90 desc
```

## Time to first data after an install

The watch events carry `command == "watch"`, so the install method comes from the `install` event of the same run:

```dql
fetch events, from: now() - 1d
| filter event.type == "CUSTOM_INFO" and startsWith(event.name, "dtwiz")
| filter step == "install" or step == "snapshot"
| summarize
    method = arrayRemoveNulls(collectDistinct(if(step == "install", subcommand))),
    hosts_s = max(toDouble(hosts)) / 1000,
    logs_s = max(toDouble(logs)) / 1000,
    services_s = max(toDouble(services)) / 1000,
    by: { executionId, version, os }
| filter arraySize(method) > 0
| fieldsAdd method = arrayFirst(method)
| summarize runs = count(), p50_hosts_s = percentile(hosts_s, 50), p50_logs_s = percentile(logs_s, 50), p50_services_s = percentile(services_s, 50), by: { method }
```

## Setup funnel

How many `setup` runs reached each step:

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and command == "setup"
| summarize runs = countDistinct(executionId), by: { step }
| sort runs desc
```

## Recommended versus chosen methods

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and command == "setup"
| filter in(step, array("recommendations_presented", "recommendations_selected"))
| summarize presented = countIf(step == "recommendations_presented"), selected = countIf(step == "recommendations_selected"), by: { option }
| sort presented desc
```

## Detected environments

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and command == "setup" and step == "analyze"
| summarize runs = count(), by: { cloud.provider, k8s.distro, os }
| sort runs desc
```

## Install script failures

```dql
fetch events, from: now() - 30d
| filter event.type == "CUSTOM_INFO" and event.name == "dtwiz install script"
| filter step == "failed" or step == "cancelled"
| summarize runs = count(), by: { script, os, error }
| sort runs desc
```
