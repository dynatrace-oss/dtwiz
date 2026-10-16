# Tasks: Install Script Self-Monitoring

## 1. install.sh: event sending

- [x] 1.1 In `scripts/install.sh`, directly after `REPO=`, compute the execution id (3 hex chars from `/dev/urandom`, fallback `000`), the event OS (header `mac`/`lin`/first 3 chars lowercased; body `darwin`/`linux`/lowercased `uname -s`), the channel (`stable`/`main`/`tag`) and the requested tag from `DTWIZ_CHANNEL`/`DTWIZ_TAG`
- [x] 1.2 Add `send_event <step> [reason]`: no-op without `DT_ENVIRONMENT`; normalize the URL (strip `.apps.` and trailing slashes); POST to `/api/v2/events/ingest` with User-Agent `dtwiz/install;s=sh;st=<step>[;v=<version>][;er=<reason>]`, Tab-Id `<execid>;o=<os>`, `dtwiz-monitoring: dtwiz-start`, JSON body per spec; `--connect-timeout 2 --max-time 3`, all output discarded, status ignored
- [x] 1.3 Pass `Authorization: Bearer $DT_PLATFORM_TOKEN` via `-H @-` on stdin, and only when the token is set
- [x] 1.4 Add `fail <reason> <message>`: print the message to stderr, `send_event fai <reason>`, `exit 1`
- [x] 1.5 Send `inv` right after the setup in 1.1, before channel handling and argument parsing
- [x] 1.6 Send `can` in the `[Nn]` confirmation branch before `exit 0`
- [x] 1.7 Send `com` immediately after the binary is moved into place, before PATH handling

## 2. install.sh: failure reasons and missing checks

- [x] 2.1 Replace each existing `exit 1` with `fail`: `unknown_argument`, `unsupported_os`, `unsupported_arch`, `curl_missing`, `release_not_found` (tag path), `binary_missing`
- [x] 2.2 Stable version resolution: fail with `release_not_found` when the resolved tag does not match `v[0-9]*` (today it silently becomes `latest`)
- [x] 2.3 Set the event version to `${VERSION#v}` once resolved, so later events carry `v=`
- [x] 2.4 Check the archive download (`download_failed`), `tar` extraction (`extract_failed`), `mkdir -p` and `mv`/`sudo mv` (`move_failed`)
- [x] 2.5 Run `sh -n scripts/install.sh` and `shellcheck scripts/install.sh`; fix new findings
- [x] 2.6 Release lookup in both paths: fail with `github_unreachable` when curl's `%{http_code}` is `000` or 3xx, otherwise `release_not_found`; do not rely on curl's exit status (22 or 56 for HTTP errors)

## 3. install.ps1: event sending

- [x] 3.1 In `scripts/install.ps1`, after `param()` and the channel translation, compute the execution id (`'{0:x3}' -f (Get-Random -Maximum 4096)`), the event OS from `$IsLinux`/`$IsMacOS` (default Windows), channel and tag
- [x] 3.2 Add `Send-InstallEvent <step> [reason]` mirroring 1.2 and 1.3 with `Invoke-WebRequest -UseBasicParsing -TimeoutSec 3`, User-Agent `dtwiz/install;s=ps1;...`, errors swallowed in `try/catch`, output discarded
- [x] 3.3 Send `inv` as the first action after 3.1
- [x] 3.4 Send `can` in the decline branch before `exit 0`
- [x] 3.5 Send `com` immediately after `Move-Item` succeeds, before the PATH update

## 4. install.ps1: failure reasons

- [x] 4.1 Add OS check: on Linux or macOS fail with `unsupported_os` and "Use install.sh on Linux and macOS", before `$env:LOCALAPPDATA` is used
- [x] 4.2 Introduce `$FailReason`, set before each stage: `unsupported_arch`, `release_not_found` (both tag and stable paths), `download_failed`, `extract_failed`, `binary_missing`, `move_failed`
- [x] 4.3 Wrap the body after `inv` in a single `try/catch` that sends `fai` with `$FailReason` and rethrows, so the user-facing error is unchanged and exactly one event is sent; keep the existing `finally` cleanup
- [x] 4.4 Set the event version to `$VersionNum` once resolved
- [x] 4.5 Run PSScriptAnalyzer on `scripts/install.ps1` if available; fix new findings
- [x] 4.6 Set `install_dir_failed` while resolving the default install directory (throw when `LOCALAPPDATA` is unset) and `prompt_failed` around `Read-Host`, so no failure is reported without a reason
- [x] 4.7 Add a `finally` to the outer `try` that sends `can` with `interrupted` when no outcome was sent (Ctrl+C)
- [x] 4.8 Add `Assert-GitHubAnswered` and call it in both release-lookup `catch` blocks: no `$_.Exception.Response` switches the reason to `github_unreachable`

## 5. Documentation

- [x] 5.1 Extend the "Self-monitoring" section in `README.md`: install scripts send events when `DT_ENVIRONMENT` is set, event name `dtwiz install script`, fields `script`, `step`, `error`, `version`, `channel`, `tag`, `os`, and the failure reason list
- [x] 5.2 Add an `[Unreleased]` entry to `CHANGELOG.md`

## 6. Verification

Capture events with a local listener (e.g. a small HTTP server printing request headers and body) and `DT_ENVIRONMENT=http://127.0.0.1:<port>`. For each case check the User-Agent, Tab-Id, routing header, body, and that each run has one `inv` and at most one outcome with the same execution id.

- [x] 6.1 No `DT_ENVIRONMENT`: no request made, install output unchanged (both scripts)
- [x] 6.2 Successful stable install: `inv` then `com` with `v=` without leading `v`, `channel=stable`, no `tag` (both scripts)
- [x] 6.3 `DTWIZ_CHANNEL=main` and `DTWIZ_TAG=snapshot-<branch>`: body `channel`/`tag` correct, `v=<x.y.z>-next` (both scripts)
- [x] 6.4 Decline the prompt: `inv` then `can`, no `er=` (both scripts)
- [x] 6.5 `DTWIZ_TAG=does-not-exist`: `fai` with `release_not_found`, no `v=` (both scripts)
- [x] 6.6 GitHub unreachable via a dead proxy (`HTTPS_PROXY=http://127.0.0.1:9`), stable and tag path: `github_unreachable`, no download attempted (`install.sh`)
- [x] 6.7 `--bogus` argument: `inv` then `fai` with `unknown_argument` (`install.sh`)
- [x] 6.8 Read-only install dir requiring sudo, password refused: `move_failed`, no `com` (`install.sh`)
- [x] 6.9 pwsh on macOS or Linux: `unsupported_os`, `o=mac`/`o=lin` (`install.ps1`)
- [x] 6.10 Windows PowerShell 5.1 and pwsh 7 on Windows: success path with `o=win`
- [x] 6.11 Unreachable `DT_ENVIRONMENT` (e.g. a blackholed IP): install completes, no output from sending, delay at most about 3 s per event
- [x] 6.12 `DT_PLATFORM_TOKEN` set: token sent as Bearer and not visible in `ps` output during the send (`install.sh`)
- [x] 6.13 Platform URL `https://<id>.apps.<domain>/`: events go to `https://<id>.<domain>`
- [x] 6.14 `install.sh` run both via `source <(...)` and `| sh`: same events, no job-control or curl output in the terminal
- [x] 6.15 `powershell -NonInteractive`: `fai` with `prompt_failed` (`install.ps1`)
- [x] 6.16 `LOCALAPPDATA` unset, no `-InstallDir`: `fai` with `install_dir_failed` (`install.ps1`)
- [x] 6.17 Pipeline stopped mid-run (Ctrl+C): `can` with `er=interrupted` (`install.ps1`)
- [x] 6.18 GitHub unreachable (pwsh 7: `HTTPS_PROXY=http://127.0.0.1:9`; Windows PowerShell 5.1: unplug network or block github.com), stable and tag path: `github_unreachable` (`install.ps1`)
