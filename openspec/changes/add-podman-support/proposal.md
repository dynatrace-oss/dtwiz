## Why

Podman is a Docker-compatible container runtime that is increasingly common on Linux and macOS (via Podman Desktop). The CLI currently only detects Docker, leaving Podman users without a recommended monitoring path despite Podman supporting the same OneAgent container deployment model.

## What Changes

- New `ContainerRuntimePodman` constant in `pkg/analyzer/analyzer.go`.
- New `detectPodman()` function in `pkg/analyzer/detect_podman.go` — mirrors Docker detection using the `podman` binary.
- `AnalyzeSystem()` runs `detectPodman()` concurrently alongside `detectDocker()`; Docker takes priority if both are present.
- New `MethodPodman` recommendation in `pkg/recommender/recommender.go` — triggered when Podman is detected without Kubernetes, same priority as Docker.
- New `InstallPodman()` installer in `pkg/installer/podman.go` — identical flow to `InstallDocker()` but runs `podman run ...`.
- New `dtwiz install podman` subcommand wired in `cmd/install.go`.

## Capabilities

### New Capabilities

- `podman-container-monitoring`: Detect Podman as a container runtime and deploy Dynatrace OneAgent as a Podman container for host + container monitoring.

### Modified Capabilities

<!-- No existing spec-level requirements change. Docker detection and installation are unaffected. -->

## Impact

- **New files:** `pkg/analyzer/detect_podman.go`, `pkg/installer/podman.go`
- **Modified files:** `pkg/analyzer/analyzer.go` (new constant + concurrent detection), `pkg/recommender/recommender.go` (new recommendation branch), `cmd/install.go` (new subcommand)
- **No breaking changes:** Docker detection and `dtwiz install docker` are unchanged.
- **Rollback:** Revert the commit — no data migration or external state involved.
