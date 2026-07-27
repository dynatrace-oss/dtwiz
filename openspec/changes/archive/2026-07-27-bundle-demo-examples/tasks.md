# Tasks: bundle-demo-examples

## 1. Add schnitzel example to the repository

- [x] 1.1 Add `examples/schnitzel/` directory with all service files (`frontend/`, `order/`, `delivery/`, `loadgenerator/`, `requirements.txt`, `README.md`)
- [x] 1.2 Add `examples/schnitzel/.gitignore` to exclude runtime artifacts (e.g. `__pycache__/`, `*.pyc`)

## 2. Publish schnitzel examples as a release asset

- [x] 2.1 Update `.goreleaser.yaml` to package the `examples/` directory as `dtwiz-examples.tar.gz` under `dist/`, publish it as an additional release asset, and keep the generated tarball out of the working tree
- [x] 2.2 Verify the release asset builds cleanly with `goreleaser build --snapshot` and the archive contains the expected files
- [x] 2.3 Verify normal platform archives do not include `examples/` and install scripts do not copy examples eagerly

## 3. Rewrite demo.go to use release asset download

- [x] 3.1 Add `bundledDemoPath()` function that returns `~/.dtwiz/examples/schnitzel` using `os.UserHomeDir()`
- [x] 3.2 Add `downloadDemoExamples(dst string)` function that constructs the `dtwiz-examples.tar.gz` release asset URL from the binary's built-in version string, downloads the archive, and extracts it to the destination path
- [x] 3.3 Remove `downloadAndExtractDemo()`, `extractZip()`, and the `demoZipURL` constant
- [x] 3.4 Remove `checkDemoExists()` (no longer needed)
- [x] 3.5 Remove unused imports from `demo.go`
- [x] 3.6 Update `InstallDemo` to call `downloadDemoExamples` when `bundledDemoPath()` does not exist, then pass the path to `InstallOtelCollectorWithProject`
- [x] 3.7 Update `InstallDemo` plan preview to show the download step only when the path is missing

## 4. Remove experimental flag gating from demo

- [x] 4.1 In `cmd/install.go`, remove `Hidden: true` and the experimental check from `installDemoCmd`
- [x] 4.2 In `cmd/setup.go`, remove the `featureflags.IsEnabled(featureflags.Experimental)` guard from the `[d]` menu option and the input handler
- [x] 4.3 In `pkg/recommender/recommender.go`, remove the experimental gate from the demo option in the formatted recommendations output

## 5. Add bundled examples as additional project scan root

- [x] 5.1 In `pkg/installer/otel/runtime_scan.go`, update `scanProjectDirs` to also scan `~/.dtwiz/examples/` (via `os.UserHomeDir()`) in addition to CWD
- [x] 5.2 Deduplicate results so a project is not listed twice if CWD is inside `~/.dtwiz/examples/`
- [x] 5.3 If `~/.dtwiz/examples/` does not exist, skip it silently

## 6. Hide install demo option when schnitzel already in project list

- [x] 6.1 In `cmd/setup.go`, check whether schnitzel is in the detected running project list before showing the `[d]` option; hide it if already detected

## 7. Unit tests

- [x] 7.1 Add `TestBundledDemoPath` to verify the function returns an absolute path ending in `schnitzel` on all platforms
- [x] 7.2 Add `TestDownloadDemoExamples` to verify that the function constructs the correct release asset URL from the version string and extracts files to the expected directory structure in a temp directory (use an HTTP test server to serve a fixture archive)
- [x] 7.3 Remove `TestCheckDemoExists` (function is deleted)
- [x] 7.4 Add a test for `scanProjectDirs` verifying that a project in the bundled examples path is returned even when CWD is a different directory
- [x] 7.5 Add a test verifying both CWD and bundled examples projects appear exactly once in combined results

## 8. Integration tests

- [x] 8.1 With `~/.dtwiz/examples/schnitzel/` absent, run `dtwiz install demo --dry-run` and verify the plan output includes the download step referencing the current version's release asset URL
- [x] 8.2 With `~/.dtwiz/examples/schnitzel/` absent, run `dtwiz install demo` and verify the directory is created with the expected files before OTel setup begins
- [x] 8.3 With `~/.dtwiz/examples/schnitzel/` already present, run `dtwiz install demo --dry-run` and verify the download step is omitted from the plan

## 9. Verification

- [x] 9.1 Run `make build` and confirm the binary builds cleanly
- [x] 9.2 Run `make lint` and confirm no new lint issues
- [x] 9.3 Run `make test` and confirm all tests pass

## 10. Snapshot/preview workflow support

- [x] 10.1 Add `SnapshotTag` variable to `pkg/version/version.go`, set to `""` by default
- [x] 10.2 Update `.goreleaser.yaml` ldflags to inject `SnapshotTag` from `GORELEASER_SNAPSHOT_TAG` env var
- [x] 10.3 Update `demoExamplesURL()` in `demo.go` to use the snapshot release URL when `version.SnapshotTag != ""`
- [x] 10.4 Update `TestDemoExamplesURL` to cover the `SnapshotTag != ""` case
- [x] 10.5 Update `preview.yml` goreleaser step to pass `GORELEASER_SNAPSHOT_TAG` so the built binary resolves the correct snapshot release URL
- [x] 10.6 Update `preview.yml` publish step to include `dtwiz-examples.tar.gz` as a release asset
- [x] 10.7 Update `release.yml` to set `GORELEASER_SNAPSHOT_TAG: ""` so the env var is always defined for goreleaser templates
