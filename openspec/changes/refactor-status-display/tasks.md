# Tasks

## 1. pkg/display package

Create the shared display package with color definitions and print helpers.

**Files:** `pkg/display/colors.go` (create), `pkg/display/print.go` (create)

- [x] 1.1 Create `pkg/display/colors.go` with exported color vars: `ColorOK` (green bold), `ColorError` (red bold), `ColorHeader` (magenta bold), `ColorMuted` (faint), `ColorLabel` (no styling)
- [x] 1.2 Create `pkg/display/print.go` with `Header(message string)`, `PrintSectionDivider()`, and `PrintStatusLine(label, message string, c *color.Color)`

## 2. Migrate pkg/analyzer

Replace local color vars in `pkg/analyzer/analyzer.go` with `pkg/display`.

**Files:** `pkg/analyzer/analyzer.go` (modify)

- [x] 2.1 Remove `colorHeader` and `colorMuted` package-level vars from `analyzer.go`
- [x] 2.2 Replace all `colorHeader.Sprint(...)` calls with `display.ColorHeader.Sprint(...)`
- [x] 2.3 Replace all `colorMuted.Sprint(...)` calls with `display.ColorMuted.Sprint(...)`

## 3. Refactor cmd/status.go

Replace local color vars, extract credential helper, fix error handling, and wire display package.

**Files:** `cmd/status.go` (modify)

- [x] 3.1 Remove the five package-level color vars (`statusOK`, `statusError`, `statusLabel`, `statusMuted`, `statusHead`)
- [x] 3.2 Replace `statusHead.Println(...)` / `statusMuted.Println(...)` section headers with `display.Header(...)` and `display.PrintSectionDivider()`
- [x] 3.3 Add `CredentialToken` struct with fields: `value`, `cliName`, `envName`, `verifyFn`, `urlFn func(string) string`
- [x] 3.4 Fix `printCredentialStatus` — add `urlFn` field to `CredentialToken` and use it for the "valid" message URL (Access Token → `installer.APIURL`, Platform Token → `installer.AppsURL`)
- [x] 3.5 Extract `printCredentialStatus(label, envURL string, token CredentialToken)` helper and replace the duplicated Access Token / Platform Token inline blocks
- [x] 3.6 Change system analysis error handling from `return nil` to `return err`
- [x] 3.7 Verify output matches the original for all credential states (not set / no env / valid / invalid) by inspecting the refactored logic

## 4. Feature Flags section in dtwiz status

Add the conditional "Feature Flags" section using `featureflags.List()` and the new display helpers.

**Files:** `cmd/status.go` (modify)

- [x] 4.1 After `fmt.Println(info.Summary())`, call `featureflags.List()` and filter to flags where `Enabled == true`
- [x] 4.2 If any flags are enabled, call `display.Header("Feature Flags")`, `display.PrintSectionDivider()`, then print each flag as its env var name followed by `enabled (<Source>)` indented by two spaces
- [x] 4.3 If no flags are enabled, omit the section entirely

## 5. Verification

- [ ] 5.1 Run `make test` — all tests pass
- [ ] 5.2 Run `make lint` — no new lint issues
- [ ] 5.3 Manual: `dtwiz status` with no credentials set — verify Connection Status section renders correctly with `✗` lines
- [ ] 5.4 Manual: `dtwiz status` with valid credentials — verify `✓ valid (<url>)` shows correct URL for each token (API URL for Access Token, Apps URL for Platform Token)
- [ ] 5.5 Manual: `dtwiz status` with `DTWIZ_ALL_RUNTIMES=true` — verify Feature Flags section appears
- [ ] 5.6 Manual: `dtwiz status` with no flags set — verify no Feature Flags section in output
- [ ] 5.7 Manual: simulate system analysis failure — verify command exits non-zero and prints `✗ system analysis failed: ...`
