## Why

The OTel Collector setup flow currently only detects and offers Python auto-instrumentation. As we expand language support (Java, Node, Go), the collector setup must list all detected app types so users can choose which to instrument. Additionally, users who only want auto-instrumentation (without collector receivers) cannot currently select "nothing" during collector setup, and the clean-install path does not guard against accidentally overwriting an existing collector.

## What Changes

- Extend the collector setup flow to detect and list all supported app types (Java, Node, Go — not just Python)
- Allow selecting NOTHING during collector app-type selection, so users who only want auto-instrumentation can skip receiver configuration
- Add an existing-collector check on the clean-install path (`dtwiz install otel`) to prevent accidental overwrites — prompt user to confirm or switch to update flow

## Capabilities

### New Capabilities
- `collector-app-listing`: Collector setup detects and presents all supported language runtimes (Python, Java, Node, Go) for instrumentation selection
- `collector-skip-selection`: Allow users to select no app type during collector setup, proceeding with collector-only installation
- `collector-overwrite-guard`: On clean install, check for an existing collector installation and prompt the user before overwriting

### Modified Capabilities

## Impact

- `pkg/installer/otel_collector.go` — `InstallOtelCollector()` and `prepareCollectorPlan()` need to detect all runtimes and present a multi-type menu
- `pkg/installer/otel.go` — orchestration may need updates to support the new selection flow
- `pkg/analyzer/detect_services.go` — runtime detection results feed into the collector setup
- `cmd/install.go` — no CLI changes, but behavior of `dtwiz install otel` changes
