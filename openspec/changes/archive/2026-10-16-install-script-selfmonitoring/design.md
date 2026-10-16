# Design: Install Script Self-Monitoring

## Context

`scripts/install.sh` (POSIX sh, usually run as `source <(curl ...)`) and `scripts/install.ps1` (usually run as `irm ... | iex`) resolve a release, download the archive from GitHub, and move the binary into place. The README quickstart exports `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` before running them, so the target environment is usually known at install time.

The binary's self-monitoring (`pkg/selfmonitoring`) posts `CUSTOM_INFO` events to `<classic-url>/api/v2/events/ingest` with a User-Agent (64-char edge capture), a Tab-Id (16-char edge capture), and `dtwiz-monitoring: dtwiz-start`. It sends even without a token, because the edge captures the headers on rejected requests too. The scripts follow the same model with a reduced field set.

Current failure handling is incomplete:

- `install.sh` has no `set -e`. The archive download, `tar`, `mkdir`, and `mv`/`sudo mv` are unchecked; a failed download is only caught later as "binary not found", and a failed `sudo mv` prints "installed".
- `install.sh` resolves the stable version from `curl -w '%{url_effective}'`. On failure curl still prints the original URL, so the version becomes `latest` and the empty check never fires.
- `install.ps1` runs with `$ErrorActionPreference = "Stop"`, so `Write-Error` throws before the `exit 1` that follows it, and `Invoke-WebRequest`/`Expand-Archive`/`Move-Item` errors propagate through a `try` that only has `finally`.

## Goals / Non-Goals

**Goals:**

- See per run whether an install was started, completed, cancelled, or failed, and why it failed.
- Same contract from both scripts, distinguishable by script.
- Every failure path reports exactly one reason; no false `completed`.

**Non-Goals:**

- Mode, debug, or other fields carried by the binary's events.
- Linking a script run to later binary runs (the execution id stays script-local).
- Fixing `exit` ending a sourced or `iex` session; this is a separate change.
- Installing stable versions through `DTWIZ_TAG`.
- An opt-out switch (the binary has none either).

## Decisions

### Separate User-Agent product `dtwiz/install`

The binary uses `dtwiz/<version>;c=<cmd>;...`. The script cannot fill `<version>` at `inv` time, and `c=` would only ever say "install script". Using `dtwiz/install` as the product keeps `dtwiz/` prefix queries working and separates script events at a glance. `s=` names the script, reusing the binary's subcommand key.

Alternative: `dtwiz/<version>;c=scr;...` with `unknown` before resolution. Rejected: it makes `dtwiz/<version>` ambiguous between binary and script.

### Reuse step codes, new reason names

Steps reuse `inv`/`com`/`fai`/`can` and their body names so existing queries on `step` keep their meaning. Failure reasons are script-specific and descriptive (`download_failed`, not the binary's broad `network_error`), because the goal is to see which stage failed. The User-Agent sends the full reason name, not an abbreviation.

Budget check (64 chars):

```text
dtwiz/install;s=ps1;st=fai;er=github_unreachable               48   no version yet
dtwiz/install;s=ps1;st=fai;v=0.12.3-next;er=download_failed    59   worst case
```

The longest reasons (`github_unreachable`, `install_dir_failed`, `release_not_found`, `unsupported_arch`, `unknown_argument`) can only happen before resolution, so they never share the header with `v=`.

### Release lookup: `github_unreachable` vs `release_not_found`

The release lookup is the script's first request to GitHub, so a single reason for "no version" would mix up two different problems: a bad or missing release, and no connection to GitHub at all. A spike of the merged reason could not be told apart. The lookup therefore reports:

- `github_unreachable` when no final answer arrived (DNS, connect, timeout, TLS, proxy).
- `release_not_found` when GitHub answered, but the requested tag, its `version.txt`, or the latest-release redirect did not yield a version. The name describes what the user asked for, a release, rather than the version string the script needs internally; a stable tag such as `v0.8.0` contains a version string and still fails, because it has no `version.txt`.

How "answered" is detected:

- sh: the HTTP status from `curl -w '%{http_code}'`. 2xx/4xx/5xx count as answered, `000` (no response) and 3xx (redirect not followed through) as unreachable. curl's exit status is not usable: an HTTP error exits with 22 or 56 depending on the curl version and protocol (observed: 56 for a 404 over HTTP/2).
- ps1: `$_.Exception.Response` in the `catch`. Windows PowerShell 5.1 (`WebException`) and PowerShell 7 (`HttpResponseException`) attach the response only when one was received.

5xx and rate-limit answers count as `release_not_found`. They are rare for release downloads and keeping them apart would need a third reason.

`download_failed` is not split the same way: losing the connection exactly between the lookup and the download is rare.

### Version: resolved archive version in the header, requested source in the body

The script already computes the version it uses for the archive name (`${VERSION#v}` in sh, `$VersionNum` in ps1). goreleaser uses the same `{{.Version}}` for the archive name and the binary's embedded version, so this value equals what the installed binary reports.

Snapshot versions are `incpatch(last tag)-next` for every branch, so they do not identify a build. The requested source therefore goes into the body: `channel` (`stable`/`main`/`tag`) and `tag` (e.g. `snapshot-feature-x`). Tag names can exceed 40 characters and stay out of the header.

Alternatives: a channel code in the header (`ch=m`/`ch=t`, 5 chars, leaves zero slack) or a truncated tag (lossy, over budget). Rejected; header-level filtering of snapshots is not needed.

### Tab-Id carries execution id and OS

`<execid>;o=<os>` mirrors the binary's Tab-Id without `m=`. The execution id is 3 hex chars:

- sh: from `od -An -N2 -tx1 /dev/urandom`, falling back to `000` if unavailable.
- ps1: `'{0:x3}' -f (Get-Random -Maximum 4096)`.

OS detection:

- sh: `uname -s` mapped `Darwin`→`mac`/`darwin`, `Linux`→`lin`/`linux`; other values are lowercased (header: first 3 chars). Detection for the event runs before the supported-OS check so `unsupported_os` events still carry the OS.
- ps1: `$IsLinux` → `lin`/`linux`, `$IsMacOS` → `mac`/`darwin`, otherwise `win`/`windows`. The variables do not exist in Windows PowerShell 5.1, which is Windows-only, so "unset" safely means Windows. The script does not enable strict mode, so reading the unset variables is safe.

### One send helper and one fail helper per script

```text
send_event <step> [reason]   silent, bounded, no-op without DT_ENVIRONMENT
fail <reason> <message>      print message to stderr, send_event fai <reason>, exit 1
```

sh:

- `send_event` builds the JSON body with `printf` from known values (no user-controlled text needs escaping beyond the tag name; the tag is restricted to characters valid in a git tag and quotes/backslashes are stripped).
- Sent synchronously with `curl -s -o /dev/null --connect-timeout 2 --max-time 3`, output discarded, exit status ignored (`|| true`).
- The `Authorization` header is passed on stdin (`-H @-`) so the token never appears in the process list.
- Synchronous rather than backgrounded: a background job in a sourced shell prints job-control noise, and ordering between `inv` and a fast `fai` is kept.
- When curl is missing, `send_event` falls back to `wget` without the `Authorization` header (wget only takes headers on the command line, which would expose the token). Without this fallback `curl_missing` could never be reported; the edge still records the headers of the unauthorized request.
- Helpers and variables use a `dtwiz_`/`DTWIZ_SM_` prefix, because a sourced script defines them in the user's shell.

ps1:

- `Send-InstallEvent` uses `Invoke-WebRequest -UseBasicParsing -TimeoutSec 3 -UserAgent ... -Headers ...` inside `try { } catch { }`, result discarded. Works on 5.1 and 7.x. Errors are suppressed with an explicit `-ErrorAction Stop` inside the `try` so the global `Stop` preference does not leak them.
- Failure handling uses a `$FailReason` variable set before each stage (`release_not_found` during resolution, switched to `github_unreachable` by `Assert-GitHubAnswered` when a request got no answer, `install_dir_failed` while resolving the default install directory, `prompt_failed` around `Read-Host`, `download_failed` around the download, `extract_failed` around `Expand-Archive`, `move_failed` around `New-Item`/`Move-Item`). Explicit checks (`unsupported_arch`, `binary_missing`, missing `LOCALAPPDATA`) set it and `throw`. A single `catch` around the body after `inv` sends `fai` with `$FailReason` and rethrows, so the user sees the same error as today and exactly one event is sent.
- Cancellation sends `can` before the existing `exit 0`.
- Ctrl+C stops the script without running `catch` blocks, but `finally` blocks still run. A `finally` on the outer `try` sends `can` with the reason `interrupted` when no outcome was sent yet. Killing the process (closing the window) runs nothing, so such runs still leave only `inv`.

### Environment URL normalization

Same rule as `installer.APIURL()`: remove `.apps.` from the host and trailing slashes. sh: `sed 's|\.apps\.|.|; s|/*$||'`. ps1: `-replace '\.apps\.', '.'` and `TrimEnd('/')`.

### `inv` placement

sh: directly after `REPO=`, before channel handling and argument parsing (channel and tag are read from env vars first, which has no side effects). ps1: first statement after `param()` and the channel translation, which only reads env vars. All later failures, including `unknown_argument`, then follow an `inv`.

### Missing failure checks

- sh download: `curl -fsSL ... -o ... || fail download_failed "..."`.
- sh extract: `tar -xzf ... || fail extract_failed "..."`.
- sh move: `mkdir -p` and `mv`/`sudo mv` each `|| fail move_failed "..."`.
- sh stable resolution: fail with `github_unreachable` when GitHub gave no answer, and with `release_not_found` when the resolved tag does not match `v[0-9]*`.
- ps1 stable resolution: require the tag to match `^v[0-9]`. The fallback chain can end on the original `.../releases/latest` request URL, which would otherwise yield the version `latest`.
- ps1 non-Windows: after OS detection, `unsupported_os` with "Use install.sh on Linux and macOS." This runs before `$env:LOCALAPPDATA` is used.

Profile and PATH updates after the binary is in place are not failure reasons; the install is reported `completed` once the binary is moved, before PATH handling, so a later PATH problem does not turn a working install into a failure.

## Risks / Trade-offs

- [Unreachable `DT_ENVIRONMENT` delays the install by up to 3 s per event, 6 s total] → Short connect timeout (2 s); accepted as the cost of synchronous sending.
- [Old curl without `-H @-` support (before 7.55, 2017)] → The send fails silently and the install continues; only the event is lost.
- [Contract now exists in Go, sh, and PowerShell and can drift] → Script contract lives in its own spec; the binary's spec is untouched. Manual verification checklist in tasks captures headers and bodies from both scripts.
- [Killed runs, and Ctrl+C in bash, produce `inv` without an outcome] → Intended: that pattern is the signal for aborted installs. PowerShell reports Ctrl+C explicitly as `can` with `interrupted`.
- [Events count against the user's ingest quota] → Two events per run; documented in the README next to the binary's note.
- [PowerShell OS variables behave differently across versions] → Explicit verification on Windows PowerShell 5.1 and pwsh 7 on Windows, Linux, and macOS.

## Migration Plan

Scripts are served from `main`, so merging deploys the change for all new installs. Rollback is a revert on `main`. No data migration; queries filtering `startsWith(event.name, "dtwiz")` pick up the new events automatically. Use `event.name == "dtwiz install script"` to separate them.

## Open Questions

None.
