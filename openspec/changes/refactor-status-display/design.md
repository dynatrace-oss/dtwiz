# Design

## Context

`cmd/status.go` defines five package-level color variables (`statusOK`, `statusError`, `statusLabel`, `statusMuted`, `statusHead`) that are identical in intent to colors independently defined in `pkg/analyzer/analyzer.go` (`colorHeader`, `colorMuted`) and scattered across 12 files in `pkg/installer/`. There is no shared source of truth for terminal colors — each package reinvents the same palette.

The `RunE` closure in `statusCmd` is 47 lines of inline logic: credential fetching, per-token validation branches, and output formatting are all mixed together. Access Token and Platform Token validation are near-identical blocks copy-pasted with minor label differences.

The `status-feature-flags` capability (active flags section in `dtwiz status`) was specified in the `feature-flags-package` change. Its implementation requires `display.Header` and `display.PrintSectionDivider` from the new display package, so it is delivered here.

## Goals / Non-Goals

**Goals:**

- Create `pkg/display` as the single source of truth for terminal color definitions and common print helpers used across `cmd/` and `pkg/`.
- Refactor `cmd/status.go` to use `pkg/display` and reduce duplication in the credential validation/display block.
- Migrate `pkg/analyzer/analyzer.go` local color vars to `pkg/display`.
- Fix the swallowed error in system analysis (`return nil` → `return err`).
- Implement the `status-feature-flags` capability: conditional feature flags section in `dtwiz status`.

**Non-Goals:**

- Migrating all 50 `color.New()` calls across `pkg/installer/` — that is a separate, larger effort.
- Theming or runtime color configuration.
- Adding `--json` output to `dtwiz status`.
- Changing the visual design of any output section beyond the spacing fix.

## Decisions

### 1. `pkg/display` — flat vars, no Theme struct

Colors are exported as flat package-level vars (`display.ColorOK`, `display.ColorError`, etc.) rather than grouped into a `Theme` struct. A struct would require callers to pass a theme instance around, adding boilerplate with no benefit at this scale. All commands share the same visual palette today; if divergent themes are needed later, the vars can be grouped then.

**Alternative considered:** `Theme` struct with predefined instances (`display.StatusTheme`, `display.InstallerTheme`). Rejected — premature abstraction; the palette is uniform across the CLI.

### 2. `printCredentialStatus()` + `CredentialToken` struct in `cmd/status.go`

The Access Token and Platform Token blocks were near-identical: three branches (not set / env not set / validate). Extracting `printCredentialStatus(label, envURL string, token CredentialToken)` with a `CredentialToken` struct (value, cliName, envName, verifyFn) eliminates the duplication.

`CredentialToken` stays in `cmd/status.go` (not in `pkg/display`) — it is a command-level concern, not a display concern.

**Alternative considered:** Two separate helpers `printAccessTokenStatus()` / `printPlatformTokenStatus()`. Rejected — still duplicates the branch logic.

### 3. Platform Token URL in `printCredentialStatus`

The original code showed `installer.APIURL(envURL)` for the Access Token and `installer.AppsURL(envURL)` for the Platform Token. The refactored `printCredentialStatus` currently always uses `installer.APIURL(envURL)` for both — this is a **bug introduced in the refactor** and must be fixed. The `CredentialToken` struct needs a `urlFn func(string) string` field (or equivalent) so each token can produce the correct URL for its "valid" message.

### 4. System analysis error propagation

Previously `dtwiz status` swallowed the system analysis error (`return nil`), making the command exit 0 even on failure. The refactored version returns the error, exiting non-zero. This is the correct behavior and consistent with how all other commands handle errors in the codebase. It is a deliberate behavioral change, not an accident.

### 5. `status-feature-flags` section uses `display` helpers

The feature flags section calls `display.Header("Feature Flags")` and `display.PrintSectionDivider()` to match the visual style of the Connection Status and System Analysis sections. The data comes from `featureflags.List()` filtered to enabled flags.

## Risks / Trade-offs

- **[Platform Token URL bug]** `printCredentialStatus` shows the wrong URL for Platform Token → Fix: add `urlFn func(string) string` to `CredentialToken` or use a separate field for the "valid" URL message.
- **[Spacing change]** Removing the trailing `\n\n` after Platform Token tightens the output. The Connection Status section previously had a blank line before System Analysis; now it doesn't. Low risk — purely cosmetic, consistent with the rest of the CLI's spacing conventions.
- **[pkg/display is production code, not test-only]** `pkg/display` ships in the binary. It adds no new dependencies (`fatih/color` is already in go.mod) and no measurable binary size impact.
