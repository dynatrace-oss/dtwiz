# Request headers

Each request also carries a compact copy of the event in its HTTP headers. Dynatrace records these headers on its side to monitor `dtwiz` itself, including for requests that your environment rejects. You can't query the header data from your environment; this page is for transparency.

| Header             | Format                                                                                          | Example                                         |
| ------------------ | ----------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `User-Agent`       | `dtwiz/<version>;c=<command>;st=<step>[;s=<subcommand>][;er=<error>]` plus step-specific fields | `dtwiz/1.14.0;c=ins;st=ist;s=otlc;f=10---;d=18` |
| `Tab-Id`           | `<executionId>;m=<mode>;o=<os>`                                                                 | `c19;m=tty;o=lin`                               |
| `dtwiz-monitoring` | always `dtwiz-start`                                                                            | `dtwiz-start`                                   |

Step-specific `User-Agent` fields:

| Field  | Step                                                    | Content                                                                                                                                                            |
| ------ | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `t=`   | `snapshot`                                              | Seconds to first data per signal, comma-separated in the fixed order `cloud,exceptions,hosts,kubernetes,logs,relationships,requests,services`; `0` means no data   |
| `f=`   | `install`                                               | One character per feature in the order `otel_config_written, host_monitoring, rum, synthetic_monitoring, rds_extensions`: `1` succeeded, `0` failed, `-` not tried |
| `d=`   | `install`                                               | Install work time in seconds                                                                                                                                       |
| `cp=`  | `analyze`                                               | Cloud providers                                                                                                                                                    |
| `kd=`  | `analyze`                                               | Kubernetes distribution                                                                                                                                            |
| `opt=` | `recommendations_presented`, `recommendations_selected` | Menu option                                                                                                                                                        |

Steps are abbreviated as `inv`, `ana`, `rpr`, `rsl`, `ist`, `snp`, `com`, `fai`, `can`. Commands, subcommands, options, errors, distributions, mode, and OS are also abbreviated (for example `c=ins` for `install`, `s=otlc` for `otel-collector`, `er=dep` for `dependency_missing`, `o=mac` for `darwin`). The install scripts send `dtwiz/install;s=<sh|ps1>;st=<step>[;v=<version>][;er=<error>]` and `Tab-Id: <executionId>;o=<os>`.

The headers are limited to 64 characters (`User-Agent`) and 16 characters (`Tab-Id`), which makes them less precise than the event body:

- Watch timings are whole seconds, at least `1` for any data, and capped at `999`.
- The install work time is whole seconds, capped at `9999`.
- Values are abbreviated, and error details such as `dependency.name` are not included.
- The `f=` positions are fixed; versions that know fewer features send a shorter `f=`.
