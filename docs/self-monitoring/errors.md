# Errors

`failed` and `cancelled` events carry `error`, and some categories add details:

| `error`                | Meaning                                                | Extra properties                                                                              |
| ---------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------------------------- |
| `user_cancelled`       | The user declined a confirmation or cancelled the menu |                                                                                               |
| `auth_error`           | The token was rejected or lacks permissions            | `auth.failure_reason`: `authentication_failed`, `invalid_token`                               |
| `config_error`         | Environment URL or token not configured                | `config.missing_fields`: `DT_ENVIRONMENT`, `DT_PLATFORM_TOKEN` (names only, never values)     |
| `dependency_missing`   | A required tool is not installed                       | `dependency.name`: `az`, `gcloud`, `java`, `node`, `npm`, `brew`, `sudo`, `winget`, `openssl` |
| `network_error`        | Your environment could not be reached                  | `network.failure_reason`: `environment_not_reachable`; `network.url`: your environment's URL  |
| `platform_unsupported` | The OS or architecture is not supported                |                                                                                               |
| `install_failed`       | Any other failure                                      | `install.step` for some failures, for example `rum_tag_fetch`                                 |

Because events are sent with your own credentials, `auth_error` and `config_error` events almost never reach your environment: a missing or rejected token means the event is rejected too. `network_error` events reach it only when part of the environment's API was reachable. Expect these categories to be rare or absent in your queries even when users hit them.
