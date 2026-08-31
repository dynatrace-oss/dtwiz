# Proposal: Combine System Analysis and Recommendation Menu

## Why

`dtwiz setup` shows a verbose "System Analysis" block followed immediately by a
"Recommendations" menu, duplicating the same detection data on one screen and forcing
users to read two sections to understand what was found and what to pick. Combining them
into a single menu — with detection context inline and undetected options shown greyed —
removes the redundancy and makes the setup flow faster to scan.

## What Changes

- **Remove** the System Analysis block from `dtwiz setup`; `dtwiz analyze` retains it unchanged.
- **Add** `DetectionInfo` to each recommendation: shown on a second line below the title,
  carrying what was detected (hostname + OS/arch + project tech stack for the host option;
  cluster name and node count for Kubernetes; account/region for AWS; subscription ID for
  Azure; project ID for GCP).
- **Add** project tech detection from the current working directory: scans for indicator
  files (`package.json`, `go.mod`, `requirements.txt`, `pom.xml`, `Cargo.toml`, `Gemfile`,
  `composer.json`, `*.csproj`, etc.) and surfaces the tech name with its full shortened
  path (home-dir prefix replaced with `~`).
- **Add** `Unavailable` recommendations: cloud providers and Kubernetes not yet connected
  appear below a "Sign in to unlock:" label in muted text, each showing the exact CLI
  command needed to unlock them (`aws configure`, `az login`, `gcloud auth login`,
  `kubectl config use-context <name>`).
- **Add** `ShortTitle` and `UnlockCommand` fields to `Recommendation` so rendering code
  reads typed data instead of parsing `DetectionInfo` strings.
- **Add** `ActionableItems()` and `FormatSetupMenu()` to the recommender package, moving
  menu rendering logic out of `cmd/setup.go`.
- Done items (e.g. OneAgent already running) are shown with a green `✓` badge in the
  setup menu.

## Capabilities

### New Capabilities

- `setup-project-detection`: Scan the current working directory for technology indicator
  files at setup time and surface the detected tech stack inline on the host recommendation.

### Modified Capabilities

- `setup-recommendations`: Detection context is now shown inline on each menu entry;
  undetected infrastructure options appear in a greyed "Sign in to unlock" section instead
  of being silently omitted.
- `system-analysis-output`: The System Analysis block is no longer printed during
  `dtwiz setup`; it remains in `dtwiz analyze` and `dtwiz status`.

## Impact

- `cmd/setup.go` — removes `info.Summary()` call; delegates menu rendering to recommender.
- `pkg/analyzer/analyzer.go` — new `ProjectTech` type and `ProjectTechs`/`ProjectDir`
  fields on `SystemInfo`; `detectProject()` wired into `AnalyzeSystem()`.
- `pkg/analyzer/detect_project.go` — new file; file-based project tech detection.
- `pkg/recommender/recommender.go` — new fields on `Recommendation`; new functions
  `ActionableItems`, `FormatSetupMenu`, `hostDetectionInfo`, `platformName`.
