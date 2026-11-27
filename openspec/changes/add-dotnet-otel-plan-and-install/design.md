# Design

## Context

See proposal.md for motivation and `add-dotnet-otel-instrumentation` (design D3, D4, D6) for the decisions this change builds on: self-contained `pkg/installer/otel/dotnet/` subpackage with no imports from `otel`, own download into a dtwiz directory, latest release (not pinned), checksum plus optional attestation, no token on disk.

Current state: step 1 delivers `dotnet.Service` (name, project file, frameworks, .NET Framework and self-contained flags, `UnsupportedReason`) and `showDotNetStep1Outcome`. The other runtimes implement `InstrumentationPlan` (`Runtime`, `PrintPlanSteps`, `Execute`) and are created in `createRuntimePlan`; Java and Go place tooling in `~/.opentelemetry/<language>/`. Java downloads a single jar with a package-level `httpClient` and has no verification; `oneagent/download.go` is the existing verify-after-download example. External commands go through `installer.CmdRunner`/`ExecLookPath` seams.

Upstream facts, verified against release v1.20.0 (2026-11-20) of `open-telemetry/opentelemetry-dotnet-instrumentation`:

- Assets: `opentelemetry-dotnet-instrumentation-<os>[-<arch>].zip` with `<os>` one of `linux-glibc`, `linux-musl` (each with `-x64` or `-arm64`), plus `macos` and `windows` (no arch suffix); also `checksums.txt`, `otel-dotnet-auto-install.sh`, `OpenTelemetry.DotNet.Auto.psm1`, `sbom.spdx.json` and a NuGet-packages zip that is not used.
- `checksums.txt` has one `<sha256>  <asset name>` line per asset (two spaces, `sha256sum` format). The releases API also lists a `sha256:` digest per asset, which matched `checksums.txt`; both come from the same origin.
- Archive layout: `VERSION` (first line is the version, e.g. `1.20.0`, then the tag and commit), `net/` (managed assemblies and `OpenTelemetry.AutoInstrumentation.StartupHook.dll`), a native profiler directory (`linux-x64`, `linux-arm64`, `linux-musl-x64`, `linux-musl-arm64`, `osx-arm64`, or `win-x64` and `win-x86` on Windows), `LICENSE`, `instrument.sh`, and the optional `additional-deps-*`/`generate-additional-deps.*` files. Windows also has `netfx/`. Native file: `OpenTelemetry.AutoInstrumentation.Native.so|dylib|dll`.
- Size: 21-23 MB compressed and up to 78 MB uncompressed on Linux (a 64 MB `.so.debug`), 64 MB for Windows, 12 MB for macOS. Archives contain no symlinks. Files are stored with mode `0644` (even `instrument.sh`).
- The official installer requires `gh` and runs two checks: `gh release verify-asset <tag> <file> --repo <repo>` and `gh attestation verify <file> --repo <repo> --signer-workflow <repo>/.github/workflows/release.yml --source-ref refs/tags/<tag>`.

## Goals / Non-Goals

**Goals:**

- A .NET `InstrumentationPlan` whose preview is accurate and side-effect free, and whose execution installs the instrumentation safely.
- All download, verification and extraction logic testable offline through injected seams.

**Non-Goals:**

- Stopping or launching processes, and environment variable generation for the launched process (step 4).
- Uninstalling the instrumentation and the ingest watch (step 5).
- Version pinning or override (umbrella open question; kept out).
- Resolving unknown target frameworks with `dotnet msbuild`.

## Decisions

### D1. Two layers: `dotnet/` does the work, `otel/` adapts

`pkg/installer/otel/dotnet/` gets, in separate files by concern: platform selection (OS, libc, arch to archive name), release lookup, download, verification (checksum, attestation), extraction with layout validation, and the install/reuse/replace orchestration (`Installer` with injected HTTP client, command runner, install root). `pkg/installer/otel/dotnet_plan.go` holds the plan type, prerequisite check, preview and `Execute` and calls the installer. *Alternative:* one file `dotnet.go` with everything (rejected: contradicts the file-per-concern and folder rules; the subpackage can be tested without `otel` types).

### D2. Plan built once, preview reads from it

`buildDotnetInstrumentationPlan(proj, ...)` runs the prerequisite check and the platform/installed-version lookups, so `PrintPlanSteps` only prints. Network is not used when building the plan: "latest vs installed" is shown as "latest release will be downloaded or reused" and the exact comparison happens in `Execute`. This keeps the preview fast and offline-safe. *Alternative:* query the release in the builder (rejected: a preview that blocks on or fails because of the network).

### D3. Prerequisite check via `dotnet --list-runtimes`

`dotnet` is located with the existing lookup seam; installed runtimes come from `dotnet --list-runtimes` (the `Microsoft.NETCore.App` and, for web services, `Microsoft.AspNetCore.App` lines) and are compared by major version with the service's target framework. Multi-target services pass when any listed framework is satisfied. Unknown frameworks pass with a note. A missing `dotnet` or no matching runtime gives per-service reasons and, if no service remains, the manual outcome. *Alternative:* require the SDK (rejected: published apps run with the runtime alone).

### D4. Install location and layout

Root: `~/.opentelemetry/dotnet/` (user profile on Windows), matching the Java and Go convention. The installed version is read from the archive's own `VERSION` file, so no separate metadata file is needed. Install builds into a sibling temporary directory, validates the layout, then swaps it in with a rename; the previous directory is kept until the swap succeeds. *Alternative:* `~/.otel-dotnet-auto` as the upstream script does (rejected: diverges from dtwiz's own convention and would collide with an upstream-managed installation).

### D5. Verification order and failure policy

Download to the temporary directory; fetch `checksums.txt` of the same release and compare the SHA-256 with the line for the chosen archive; if `gh` is on PATH and `gh auth status` succeeds, run the same two `gh` checks as the official installer (release verification and attestation with the release workflow and tag ref); extract only after all applicable checks pass. Each check failure aborts with a stage-named error; nothing falls back. `gh` absence and not-signed-in are silent (debug log only). The stronger/weaker rule from the umbrella is thereby unchanged.

### D6. Extraction safety

Entries are written only below the temporary directory (path-clean and prefix check, no symlink or absolute-path entries), and total uncompressed size is bounded (limit 512 MB, well above the 78 MB seen) to defend against zip bombs. No file needs an executable bit: the native profiler is loaded by the runtime and dtwiz computes the activation environment itself instead of running `instrument.sh`, so permissions are written as `0644` files and `0755` directories. Layout validation checks a fixed list per platform, kept in one place and tested: `VERSION`, `net/OpenTelemetry.AutoInstrumentation.StartupHook.dll`, and the native library of the platform's directory (all native directories, `win-x64` and `win-x86` on Windows). An upstream layout change fails loudly.

The optional `additional-deps-*` files (`generate-additional-deps.*`, the copy plan and per-framework deps files) are extracted but unused: the upstream activation script sets only the startup hook and profiler variables, and they are meant for scenarios with assembly conflicts that step 4 does not cover.

### D7. Execute ends in the manual outcome until step 4

`Execute` = (install or reuse) then the existing manual-steps outcome with a line saying that automatic launch is not available yet. This mirrors Go's "toolchain not installed" outcome and keeps the flag-gated flow shippable without the launch code. Ctrl+C is handled with a context like Go's `Execute`, and the temporary directory is cleaned on cancel.

### D8. Where the token goes

Steps 2 and 3 launch nothing, so no token or exporter environment is produced here; no credential is read by the plan. The "token only through the launched process's environment, never in the preview or on disk" rule stays in the umbrella spec and gets its tests in step 4. Per the umbrella design the exporter points at the local collector, so the application process may not need the token at all; step 4 decides that.

### D9. Tests with a fake release

The installer takes an `http.Client` (or `RoundTripper`) and a command runner, so tests serve a generated zip and checksum file from `httptest`, cover mismatch, missing checksum, traversal entry, layout failure, interrupted install and reuse, and stub `gh`. Platform selection is a pure function table-tested for every supported and unsupported combination, with libc detection behind a seam. No test uses the network or the real home directory (install root is injected).

## Risks / Trade-offs

- **Unpinned latest breaks on an upstream change** -> layout validation, reuse of an existing installation when unreachable, layout list kept in one tested place.
- **Release asset or checksum file naming differs from the assumed names** -> verify against a real release before implementing (task); selection logic isolated in one function.
- **GitHub API rate limiting for unauthenticated lookups** -> fall back to the release redirect URL for "latest"; report rate-limit errors as user-fixable, with reuse offered if an installation exists.
- **libc detection is heuristic on Linux** -> detect through the loader/`ldd` output behind a seam; an undetectable libc is reported as unsupported with the manual outcome rather than guessed.
- **Rename-based swap is not atomic across filesystems or when files are in use on Windows** -> temporary directory is a sibling of the target; on Windows a failed swap leaves the old installation and reports the cause.
- **Prerequisite check relies on `dotnet --list-runtimes` output format** -> parse tolerantly, treat a parse failure like "unknown" with a note, not as a hard failure.

## Migration Plan

Ships dark behind `DTWIZ_DOTNET_OTEL`. The only machine change is `~/.opentelemetry/dotnet/`, created on confirmed execution. Rollback: leave the flag off, or delete the directory; step 5 adds `dtwiz uninstall` support.

## Open Questions

None blocking. Possible later options, outside this change: a version override and use of the additional-deps files for apps with assembly conflicts.
