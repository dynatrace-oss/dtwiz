## ADDED Requirements

### Requirement: Podman runtime detection
The system SHALL detect a running Podman daemon by probing the `podman` binary. Detection SHALL collect the server version, running container count, and variant (e.g., Podman Desktop, Podman Machine).

#### Scenario: Podman detected, Docker absent
- **WHEN** `podman version` succeeds and `docker version` fails
- **THEN** `ContainerRuntime` is set to `"podman"` and `PodmanInfo.Available` is `true`

#### Scenario: Both Docker and Podman detected
- **WHEN** both `docker version` and `podman version` succeed
- **THEN** `ContainerRuntime` is set to `"docker"` and Podman is not surfaced as the primary runtime

#### Scenario: Podman binary absent
- **WHEN** `podman` is not on PATH or `podman version` fails
- **THEN** `PodmanInfo.Available` is `false` and `ContainerRuntime` remains unchanged

### Requirement: Podman monitoring recommendation
The system SHALL recommend deploying Dynatrace OneAgent as a Podman container when Podman is the detected container runtime and Kubernetes is not present.

#### Scenario: Podman without Kubernetes
- **WHEN** `ContainerRuntime` is `"podman"` and `Orchestrator` is `"none"`
- **THEN** a recommendation with method `"podman"` and priority `20` is generated

#### Scenario: Podman with Kubernetes
- **WHEN** `ContainerRuntime` is `"podman"` and `Orchestrator` is `"kubernetes"`
- **THEN** no Podman-specific recommendation is generated

### Requirement: Podman OneAgent installation
The system SHALL deploy Dynatrace OneAgent as a Podman container using `podman run` with the same host-access flags as the Docker installer.

#### Scenario: Successful installation
- **WHEN** `dtwiz install podman` is run with valid credentials and Podman is available
- **THEN** a container named `dynatrace-oneagent` is started via `podman run --detach --privileged --pid=host --net=host --restart=always -v /:/mnt/root`

#### Scenario: Dry run
- **WHEN** `dtwiz install podman --dry-run` is run
- **THEN** the `podman run` command is printed and no container is started

#### Scenario: Podman not available at install time
- **WHEN** `dtwiz install podman` is run but the Podman daemon is not accessible
- **THEN** the command exits with an error indicating Podman is unavailable
