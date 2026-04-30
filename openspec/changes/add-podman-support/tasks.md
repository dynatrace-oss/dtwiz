## 1. Analyzer: Podman detection

- [x] 1.1 Add `ContainerRuntimePodman ContainerRuntime = "podman"` constant to `pkg/analyzer/analyzer.go` alongside `ContainerRuntimeDocker`
- [x] 1.2 Add `PodmanInfo` struct to `pkg/analyzer/analyzer.go` mirroring `DockerInfo` (fields: `Available bool`, `ServerVersion string`, `Variant string`, `RunningContainerCount int`)
- [x] 1.3 Add `Podman *PodmanInfo` field to `SystemInfo` in `pkg/analyzer/analyzer.go` alongside the `Docker` field
- [x] 1.4 Create `pkg/analyzer/detect_podman.go` with `detectPodman() *PodmanInfo` — probe `podman version --format {{.Server.Version}}`, collect running container count via `podman ps -q`, detect variant via `podman info --format {{.Host.Distribution.Distribution}}` (e.g., "Podman Desktop", "Podman Machine")
- [x] 1.5 In `AnalyzeSystem()` (`pkg/analyzer/analyzer.go`), add a concurrent `run()` block for `detectPodman()` that sets `info.Podman` and, only if `info.ContainerRuntime != ContainerRuntimeDocker`, sets `info.ContainerRuntime = ContainerRuntimePodman`

## 2. Analyzer: tests

- [x] 2.1 Add unit tests in `pkg/analyzer/detect_podman_test.go` covering: Podman available (mock `podman version` success), Podman absent (mock failure), Docker+Podman both present (Docker wins)

## 3. Recommender: Podman recommendation

- [x] 3.1 Add `MethodPodman IngestMethod = "podman"` constant to `pkg/recommender/recommender.go`
- [x] 3.2 Add recommendation branch in `GenerateRecommendations()` (`pkg/recommender/recommender.go`): when `system.ContainerRuntime == analyzer.ContainerRuntimePodman && system.Orchestrator != analyzer.OrchestratorKubernetes`, append a `Recommendation` with `Method: MethodPodman`, `Priority: 20`, title "Podman host + containers (via OneAgent)", description referencing Podman, prerequisites `["Podman daemon access", "Dynatrace API token"]`, steps `["dtwiz install podman"]`
- [x] 3.3 Add tests in `pkg/recommender/recommender_test.go`: `TestGenerateRecommendations_PodmanOnly` (Podman detected, no K8s → MethodPodman recommended), `TestGenerateRecommendations_PodmanWithKubernetes` (Podman + K8s → no Podman recommendation), `TestGenerateRecommendations_DockerWinsOverPodman` (Docker runtime → no Podman recommendation)

## 4. Installer: Podman installer

- [x] 4.1 Create `pkg/installer/podman.go` with `isPodmanAvailable() bool` — check `podman` on PATH and run `podman info --format {{.Host.Hostname}}` as a daemon ping
- [x] 4.2 Implement `InstallPodman(envURL, token string, dryRun bool) error` in `pkg/installer/podman.go` — identical flow to `InstallDocker()` substituting `podman` for `docker` in all exec calls; dry-run output references `podman` binary; log line reads `podman logs -f dynatrace-oneagent`

## 5. CLI: new subcommand

- [x] 5.1 Add `installPodmanCmd` in `cmd/install.go` modelled on `installDockerCmd` (lines 71–91): `Use: "podman"`, calls `installer.InstallPodman(envURL, accessTok, installDryRun)`, calls `installer.WatchIngest` on success when not dry-run
- [x] 5.2 Register `installPodmanCmd` via `installCmd.AddCommand(installPodmanCmd)` in the `init()` block of `cmd/install.go`

## 6. Verification

- [x] 6.1 Run `make build` — binary compiles cleanly
- [x] 6.2 Run `make test` — all existing and new tests pass
- [x] 6.3 Run `make lint` — no new lint issues
- [x] 6.4 Manual dry-run: `dtwiz install podman --dry-run` prints the `podman run` command without executing it
