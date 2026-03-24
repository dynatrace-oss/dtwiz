## Context

The OTel Collector setup flow in `InstallOtelCollector()` currently hard-codes Python detection: it calls `exec.LookPath("python3")` and, if found, runs `DetectPythonPlan()`. The analyzer's `detect_services.go` already detects java, node, python3, and go runtimes. The collector template (`otel.tmpl`) uses a generic OTLP receiver that works for all languages — the language-specific part is only in which auto-instrumentation installer gets invoked after collector setup.

The collector setup also already checks for running collectors in the `execute()` method and prompts the user. However, the check only fires during execution — not during the plan phase. On the `dtwiz install otel` path (clean install), there is no pre-check to warn the user before they commit.

## Goals / Non-Goals

**Goals:**
- Make collector setup language-agnostic: detect all runtimes, present a selection menu
- Support a "None" selection for users who want collector-only without auto-instrumentation
- Guard clean-install path against overwriting an existing collector

**Non-Goals:**
- Multi-select of app types (meeting notes: single-app instrumentation only for now)
- Changing the collector binary, template, or exporter configuration
- Language-specific instrumentation logic (handled by per-language changes)

## Decisions

**1. Runtime detection uses `exec.LookPath` directly (not the analyzer)**
The collector setup needs to know which runtimes are available for instrumentation. While `detect_services.go` uses `which` via shell, the existing Python code in `InstallOtelCollector()` uses `exec.LookPath("python3")` directly. We'll follow the established pattern: `exec.LookPath` for each supported binary (`python3`/`python`, `java`, `node`, `go`). This avoids adding an analyzer dependency to the installer package.

Alternative: Call `detectServices()` from the analyzer — rejected because it returns a flat `[]string` mixing runtimes with daemons (nginx, postgres), and would create a cross-package dependency.

**2. Single-select menu with "None" option**
Present detected runtimes as a numbered menu with an additional "None — collector only" option. User picks one. This matches the meeting requirement of single-app instrumentation only.

Alternative: Multi-select with checkboxes — rejected per meeting notes (deferred to later).

**3. Pre-plan collector existence check**
Before calling `prepareCollectorPlan()`, check for existing collector installations (running processes via `findRunningOtelCollectors()` + directory existence). If found, present options: overwrite, switch to `dtwiz update otel`, or abort.

Alternative: Only check during execution (current behavior) — rejected because by that point the user has already seen the full plan and confirmed.

## Risks / Trade-offs

- [Menu UX complexity] → Keep menu simple: one numbered list, one prompt. No nested menus.
- [Runtime detected but installer not implemented yet] → Show runtime in list but mark as "coming soon" if the corresponding `otel-<lang>` installer doesn't exist yet. Don't crash.
- [Existing collector detection false positives] → Reuse the proven `findRunningOtelCollectors()` + directory check. Accept that edge cases (manually installed collectors in non-standard paths) may not be caught.
