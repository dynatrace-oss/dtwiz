# Design: bundle-demo-examples

## Context

The demo install flow currently fetches the schnitzel app from `github.com/dietermayrhofer/schnitzel` at runtime. This means the demo breaks if that repo moves or is unavailable, and the version of the example app is not tied to the dtwiz version. The schnitzel app is now maintained inside the dtwiz repo under `examples/schnitzel/`, so it can ship as part of the dtwiz release.

## Goals / Non-Goals

**Goals:**

- Publish `examples/schnitzel/` as a release asset and download it on demand so the demo does not depend on any third-party repository
- Save to a fixed location when the path does not exist on disk
- Remove the dependency on the third-party schnitzel repository

**Non-Goals:**

- Starting the demo services automatically (users still run services manually)
- Updating examples in place when dtwiz is upgraded (a re-download only happens when the path is absent)
- Supporting multiple demo apps (only schnitzel for now)

## Decisions

### Release asset download over `go:embed` or third-party download

Publishing all examples together as `dtwiz-examples.tar.gz` and downloading it on demand keeps the binary small for all users. Users who never run the demo get no extra weight. The download only happens when `~/.dtwiz/examples/schnitzel/` is absent. The URL is built from the binary's built-in version string, so the downloaded files always match the running version. Schnitzel is one example inside the archive; future examples can be added to the same asset without changing the download mechanism.

Alternatives considered:

- **`go:embed`**: Demo files baked into every binary. Works offline after first extraction. Rejected because it adds demo overhead to all users, including those who never use the demo.
- **GoReleaser archive + install script copy**: examples ship in the release archive and install scripts copy them to `~/.dtwiz/examples/`. Works for users who use the install script, but fails silently for users who download the binary manually. Also adds complexity to the install scripts on all platforms.
- **Download the full release archive**: if `~/.dtwiz/examples/schnitzel/` is missing, download the full release archive and extract examples from it. Rejected in favor of a small dedicated asset (`dtwiz-examples.tar.gz`) that contains only the example apps.
- **Write archive to disk then delete after extraction**: download `dtwiz-examples.tar.gz` to a temp file, extract it, then delete the archive. Rejected in favor of streaming the HTTP response body directly into the tar extractor — no temp file is ever written, so no cleanup step is needed.

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

- **Binary size**: no change. Demo files are not embedded in the binary. They are downloaded only when needed.
- **Network requirement when path is absent**: downloading the release asset requires a network call if `~/.dtwiz/examples/schnitzel/` is missing. This is not a new limitation: the demo already requires network access for OTel Collector setup and Dynatrace ingestion.
- **Stale examples after upgrade**: if a user upgrades dtwiz and the old `~/.dtwiz/examples/schnitzel/` already exists, it will not be automatically refreshed. The new binary will download the updated asset only when the path is absent. For now this is acceptable; version checking is out of scope.
- **User edits to downloaded files**: if a user modifies `~/.dtwiz/examples/schnitzel/`, those changes persist. A re-download only happens when the path is absent. This is the intended behavior.
