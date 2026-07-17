# Design: bundle-demo-examples

## Context

The demo install flow currently fetches the schnitzel app from `github.com/dietermayrhofer/schnitzel` at runtime. This means the demo breaks if that repo moves or is unavailable, and the version of the example app is not tied to the dtwiz version. The schnitzel app is now maintained inside the dtwiz repo under `examples/schnitzel/`, so it can ship as part of the binary itself.

## Goals / Non-Goals

**Goals:**

- Embed `examples/schnitzel/` in the dtwiz binary so the demo works without any network access to fetch the app
- Extract to a fixed, predictable location when the path does not exist on disk
- Remove all network-dependent demo setup code

**Non-Goals:**

- Starting the demo services automatically (users still run services manually)
- Updating examples in place when dtwiz is upgraded (re-extraction replaces them)
- Supporting multiple demo apps (only schnitzel for now)

## Decisions

### `go:embed` over archive bundling or download fallback

Embedding `examples/schnitzel/` directly in the binary using Go's `embed` package means the binary is fully self-contained. There is nothing to copy, download, or lose. As long as the dtwiz binary is on the machine, the demo can always extract the example app from itself.

Alternatives considered:
- **Goreleaser archive + install script copy**: examples ship in the release archive and install scripts copy them to `~/.dtwiz/examples/`. Works for users who use the install script, but fails silently for users who download the binary manually. Also adds complexity to the install scripts on all platforms.
- **Download fallback from own release archive**: if `~/.dtwiz/examples/schnitzel/` is missing, download the release archive and extract examples from it. Requires a network call and downloading the full archive just to get a few KB of Python files.

### Examples location: `~/.dtwiz/examples/` over CWD

The extracted files are placed at a fixed home directory path. CWD at install time and CWD when `dtwiz setup` runs later are almost never the same directory. `os.UserHomeDir()` resolves consistently across macOS, Linux, and Windows.

Alternatives considered:
- **Next to the binary**: not always writable (e.g. `/usr/local/bin`), and varies by install method
- **CWD**: unpredictable, breaks across different terminal sessions

### Run demo in place vs. copying to CWD

The demo is instrumented directly from `~/.dtwiz/examples/schnitzel/`. This avoids any copy step and removes the need to track whether a local `./schnitzel/` exists. `InstallOtelCollectorWithProject` takes an absolute path, so it works with any location.

Alternatives considered:
- **Copy to CWD**: adds complexity, creates user-visible state in arbitrary directories, requires checking if CWD copy is stale

## Risks / Trade-offs

- **Binary size**: embedding the schnitzel Python files adds a small amount to the binary size. The schnitzel app is a handful of small text files, so the impact is negligible.
- **Stale examples after upgrade**: if a user upgrades dtwiz and the old `~/.dtwiz/examples/schnitzel/` already exists, it will not be automatically refreshed. The new binary embeds the new version, but the on-disk copy stays until the user deletes it or the demo detects a version mismatch. For now this is acceptable; version checking is out of scope.
- **User edits to extracted files**: if a user modifies `~/.dtwiz/examples/schnitzel/`, those changes persist. A re-extraction only happens when the path is absent. This is the intended behavior.
