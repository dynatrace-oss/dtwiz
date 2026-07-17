# Proposal: bundle-demo-examples

## Why

The demo install flow downloads the schnitzel app from an external GitHub repository at runtime, creating a hard dependency on a third-party repo and requiring a network call during demo setup. Embedding the example app directly in the dtwiz binary makes the demo self-contained, removes the external dependency, and ties the example version to the tool version.

## What Changes

- The `examples/schnitzel/` directory is added to the dtwiz repository under `examples/`
- A new Go package exposes `examples/schnitzel/` as an embedded filesystem using `go:embed`
- When `~/.dtwiz/examples/schnitzel/` does not exist, `InstallDemo` extracts it from the embedded filesystem instead of downloading from the internet
- `InstallDemo` reads from `~/.dtwiz/examples/schnitzel/` directly and instruments it in place. There is no copy to the current working directory
- All download logic is removed: `downloadAndExtractDemo`, `extractZip`, and `demoZipURL` are deleted
- The `checkDemoExists` function is removed (no longer needed)

## Capabilities

### New Capabilities

- `bundle-examples`: The schnitzel example app is embedded in the dtwiz binary and extracted to `~/.dtwiz/examples/` on demand

### Modified Capabilities

- `install-demo`: Requirements change: the demo app is no longer downloaded from a URL or copied to the working directory; it is extracted from the binary if needed and read from `~/.dtwiz/examples/schnitzel/`
- `otel-project-scan`: `~/.dtwiz/examples/` is added as an additional scan root so the demo app appears in the project list regardless of the user's working directory

## Impact

- `examples/`: new directory containing the schnitzel app source files
- `examples/embed.go`: new Go package that exposes the embedded filesystem
- `pkg/installer/otel/demo.go`: download functions removed, `InstallDemo` rewritten to extract from embedded FS and use bundled path
- No changes to `.goreleaser.yaml`, `scripts/install.sh`, or `scripts/install.ps1`
- No changes to CLI surface, flags, or auth
