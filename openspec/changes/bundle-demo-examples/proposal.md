# Proposal: bundle-demo-examples

## Why

The demo install flow downloads the schnitzel app from an external GitHub repository at runtime, creating a hard dependency on a third-party repo and requiring a network call during demo setup. Publishing the example app as a dtwiz release asset removes the external dependency, ties the example version to the tool version, and keeps the dtwiz binary small. Demo files are only fetched by users who actually run the demo.

## What Changes

- The `examples/schnitzel/` directory is added to the dtwiz repository under `examples/`
- `.goreleaser.yaml` is updated to publish `dtwiz-examples.tar.gz` as an additional release asset
- When `~/.dtwiz/examples/schnitzel/` does not exist, `InstallDemo` downloads `dtwiz-examples.tar.gz` from the current dtwiz GitHub release and extracts it to `~/.dtwiz/examples/`
- `InstallDemo` reads from `~/.dtwiz/examples/schnitzel/` directly and instruments it in place. There is no copy to the current working directory
- All third-party download logic is removed: `downloadAndExtractDemo`, `extractZip`, and `demoZipURL` are deleted and replaced with version-pinned release asset download
- The `checkDemoExists` function is removed (no longer needed)

## Capabilities

### New Capabilities

- `bundle-examples`: All example apps are published together as a dtwiz release asset (`dtwiz-examples.tar.gz`) and downloaded on demand to `~/.dtwiz/examples/` when the demo command is run. Schnitzel is one of the examples inside the archive.

### Modified Capabilities

- `install-demo`: Requirements change: the demo app is no longer downloaded from a third-party URL or copied to the working directory; it is downloaded from the current dtwiz release asset if needed and read from `~/.dtwiz/examples/schnitzel/`
- `otel-project-scan`: `~/.dtwiz/examples/` is added as an additional scan root so the demo app appears in the project list regardless of the user's working directory

## Impact

- `examples/`: new directory containing the schnitzel app source files
- `.goreleaser.yaml`: updated to publish `dtwiz-examples.tar.gz` as a release asset
- `pkg/installer/otel/demo.go`: third-party download functions removed, `InstallDemo` rewritten to download from the dtwiz release asset and use the bundled path
- No changes to `scripts/install.sh` or `scripts/install.ps1`
- No changes to CLI surface, flags, or auth
