# Design

## Context

The analyzer runs detection functions concurrently via a `run()` helper that fans out goroutines and joins them with a mutex-protected `SystemInfo` struct. Docker detection lives in `pkg/analyzer/detect_docker.go` and sets `info.ContainerRuntime = ContainerRuntimeDocker` when the `docker` binary responds. The recommender then branches on `ContainerRuntime` to generate a `MethodDocker` recommendation, and `pkg/installer/docker.go` handles the `dtwiz install docker` subcommand.

Podman is CLI-compatible with Docker: `podman version`, `podman ps`, `podman run` accept the same flags. The installer command (`podman run --detach --privileged --pid=host --net=host -v /:/mnt/root ...`) is identical except for the binary name.

## Goals / Non-Goals

**Goals:**

- Detect Podman via the `podman` binary, mirroring Docker detection.
- Set `ContainerRuntimePodman` when Podman is available and Docker is not.
- Generate a `MethodPodman` recommendation (same priority as Docker).
- Install OneAgent via `podman run` with the same flags as `docker run`.
- Expose `dtwiz install podman` as a new CLI subcommand.

**Non-Goals:**

- Detecting rootless vs. rootful Podman — treated identically.
- Supporting both Docker and Podman simultaneously — Docker wins when both are present.
- Podman-specific variant detection beyond what `podman version` provides.

## Decisions

### 1. Docker takes priority when both are present

`detectDocker()` and `detectPodman()` run concurrently. Both write to `info.ContainerRuntime` under a mutex. The write order is non-deterministic, so we add an explicit check: only set `ContainerRuntimePodman` if `ContainerRuntimeDocker` has not already been set.

**Alternative considered:** Run Podman detection only if Docker is absent. Rejected — sequential detection would slow analysis; the mutex guard is simpler and keeps the fan-out pattern consistent.

### 2. Separate `detect_podman.go` file, mirroring `detect_docker.go`

Podman detection is a new file rather than extending `detect_docker.go`. This keeps each runtime's detection isolated and easy to remove independently.

### 3. `PodmanInfo` struct mirrors `DockerInfo`

A new `PodmanInfo` struct (same fields: `Available`, `ServerVersion`, `Variant`, `RunningContainerCount`) is added to `SystemInfo` alongside `DockerInfo`. This keeps the JSON output consistent and allows future Podman-specific fields without touching Docker's struct.

### 4. `InstallPodman()` in a new `podman.go`, not merged into `docker.go`

Same rationale as detection: isolated, independently removable. The implementation is a near-copy with `podman` substituted for `docker` and container/log messages updated accordingly.

## Risks / Trade-offs

- **[Non-deterministic priority when both runtimes present]** → Mitigated by the explicit "don't overwrite Docker" guard in the mutex block. Docker always wins.
- **[Podman Desktop on Mac may also expose a Docker socket]** → In that case `detectDocker()` may succeed via the Docker-compatible socket, and Podman detection is skipped correctly (Docker wins). No special handling needed.
- **[Rootless Podman limitation]** → `--privileged --pid=host --net=host` behave differently under rootless Podman on Linux. Known limitation, out of scope for this change.
