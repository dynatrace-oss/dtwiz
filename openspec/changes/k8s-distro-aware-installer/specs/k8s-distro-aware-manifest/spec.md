# K8s Distro-Aware Manifest

## ADDED Requirements

### Requirement: InstallKubernetes accepts clusterName and distribution parameters

The system SHALL accept a `clusterName string` and a `distro string` parameter in `InstallKubernetes()`. When `clusterName` is empty it SHALL be derived from the current kubectl context. When `distro` is empty it SHALL default to `"kubernetes"` behavior.

#### Scenario: clusterName and distro wired from detected KubernetesInfo

- **GIVEN** the analyzer has detected a Kubernetes cluster with non-empty `Cluster` and `Distribution` in `KubernetesInfo`
- **WHEN** `installKubernetesCmd` or `setup` runs
- **THEN** both `Cluster` and `Distribution` SHALL be passed to `InstallKubernetes()`, avoiding a redundant kubectl call inside the installer

#### Scenario: Empty clusterName falls back to kubectl context

- **GIVEN** `InstallKubernetes()` is called with an empty `clusterName` string
- **WHEN** the installer runs
- **THEN** the cluster name SHALL be derived from `kubectl config view` and sanitized for use as a Kubernetes resource name

#### Scenario: Empty distro falls back to default

- **GIVEN** `InstallKubernetes()` is called with an empty `distro` string
- **WHEN** the manifest is rendered
- **THEN** the rendered manifest SHALL match the `"kubernetes"` (generic) distro behavior — KSPM enabled, no annotations, no kubeletPath override

### Requirement: KSPM block included only for standard Linux distributions

The system SHALL include the `kspm.mappedHostPaths` block and `kspmNodeConfigurationCollector` image reference in the rendered manifest only for distributions where standard Linux host paths are accessible.

#### Scenario: KSPM rendered for EKS

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"EKS"`
- **THEN** rendered manifest contains `mappedHostPaths` with all 7 paths and `kspmNodeConfigurationCollector` image ref

#### Scenario: KSPM rendered for AKS

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"AKS"`
- **THEN** rendered manifest contains `mappedHostPaths` block

#### Scenario: KSPM rendered for generic kubernetes

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"kubernetes"` or empty
- **THEN** rendered manifest contains `mappedHostPaths` block

#### Scenario: KSPM rendered for local dev distros (minikube, kind, k3s)

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"minikube"`, `"kind"`, or `"k3s"`
- **THEN** rendered manifest contains `mappedHostPaths` block, no privileged or readonly-volume annotations, and no `kubeletPath` field — identical behavior to generic `"kubernetes"`

#### Scenario: KSPM omitted for GKE

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"GKE"` or `"GKE-Autopilot"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for OpenShift

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"OpenShift"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for Bottlerocket

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"EKS-Bottlerocket"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for RKE

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"RKE"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for IKS

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"IKS"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for TKGI

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"TKGI"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

### Requirement: OpenShift manifests carry privileged annotation on both DynaKubes

The system SHALL add `feature.dynatrace.com/oneagent-privileged: "true"` to the metadata annotations of both DynaKube objects when the distribution is OpenShift.

#### Scenario: Annotation present on both DynaKubes

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"OpenShift"`
- **THEN** both the monitoring DynaKube and the agents DynaKube in the rendered manifest contain `feature.dynatrace.com/oneagent-privileged: "true"`

#### Scenario: Annotation absent on non-OpenShift distros

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is any value other than `"OpenShift"`
- **THEN** neither DynaKube in the rendered manifest contains `oneagent-privileged`

### Requirement: Bottlerocket manifests SHALL NOT carry the injection-readonly-volume annotation

The `feature.dynatrace.com/injection-readonly-volume: "true"` annotation was required for Dynatrace Operator **0.12.0+** and **< 1.7.0** to make the injected CSI volume read-only on Bottlerocket nodes. Starting with Operator **1.7.0**, read-only CSI volumes are injected automatically — no annotation is needed. dtwiz targets Operator ≥ 1.7.0; the annotation MUST NOT be added to any DynaKube manifest regardless of distro.

Reference: [Dynatrace Docs — injection-readonly-volume](https://docs.dynatrace.com/docs/ingest-from/setup-on-k8s/guides/networking-security-compliance/advanced-security-configurations/injection-readonly-volume)

#### Scenario: Annotation absent on EKS-Bottlerocket

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"EKS-Bottlerocket"`
- **THEN** neither DynaKube in the rendered manifest contains `feature.dynatrace.com/injection-readonly-volume`

#### Scenario: Annotation absent on all other distros

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is any value
- **THEN** no DynaKube in the rendered manifest contains `feature.dynatrace.com/injection-readonly-volume`

### Requirement: Non-standard kubelet paths set in manifest for IKS and TKGI

The system SHALL set the appropriate `kubeletPath` in the rendered DynaKube spec when the distribution uses a non-default kubelet data directory.

#### Scenario: IKS kubeletPath set

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"IKS"`
- **THEN** rendered manifest contains `kubeletPath: /var/data/kubelet`

#### Scenario: TKGI kubeletPath set

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"TKGI"`
- **THEN** rendered manifest contains `kubeletPath: /var/vcap/data/kubelet`

#### Scenario: kubeletPath absent for standard distros

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is GKE, GKE-Autopilot, EKS, AKS, OpenShift, RKE, generic kubernetes, minikube, kind, or k3s
- **THEN** rendered manifest does NOT contain `kubeletPath`

### Requirement: Templates block belongs only in DynaKube #2 (agents), not DynaKube #1 (monitoring)

Per the Dynatrace Kubernetes onboarding guidelines for each supported distribution, DynaKube #1 (monitoring) SHALL contain only `activeGate` with the `kubernetes-monitoring` capability and the optional KSPM block. Extension and image templates (EEC, OTel collector, log module) belong exclusively in DynaKube #2 (agents), alongside `telemetryIngest`, `logMonitoring: {}`, and `extensions`. DynaKube #1 SHALL NOT carry these fields. When KSPM is enabled, only `kspmNodeConfigurationCollector` appears under `templates` in DynaKube #1.

#### Scenario: DynaKube #1 templates limited to kspmNodeConfigurationCollector

- **GIVEN** `InstallKubernetes()` is called with a KSPM-enabled distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #1 `templates` contains only `kspmNodeConfigurationCollector`; no EEC, OTel, or logModule entries

#### Scenario: DynaKube #1 has no templates block when KSPM disabled

- **GIVEN** `InstallKubernetes()` is called with a non-KSPM distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #1 has no `templates` block at all

#### Scenario: DynaKube #2 carries all extension templates

- **GIVEN** `InstallKubernetes()` is called for any distro
- **WHEN** the manifest is rendered
- **THEN** DynaKube #2 `templates` contains EEC, OTel collector, and log module image refs; `telemetryIngest`, `logMonitoring: {}`, and `extensions.prometheus: {}` are also present in DynaKube #2

### Requirement: Helm CSI driver disabled on GKE Autopilot

The system SHALL pass `--set csidriver.enabled=false` to the Helm install/upgrade command when the distribution is `"GKE-Autopilot"`. GKE Autopilot blocks privileged containers and write-mode hostPath mounts, both of which are required by the Dynatrace CSI driver DaemonSet. The CSI driver is not needed when `applicationMonitoring: {}` is used.

#### Scenario: CSI driver disabled on GKE Autopilot install

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"GKE-Autopilot"`
- **THEN** the Helm command includes `--set csidriver.enabled=false`

#### Scenario: CSI driver enabled on all other distros

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is any value other than `"GKE-Autopilot"`
- **THEN** the Helm command does NOT include `--set csidriver.enabled=false`

### Requirement: ClusterRoleBinding included in all rendered manifests

The system SHALL include a `ClusterRoleBinding` for `dynatrace-kubernetes-monitoring-sensitive` in every rendered manifest, binding to the `dynatrace-activegate` service account.

#### Scenario: ClusterRoleBinding present

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** any distro string is passed
- **THEN** rendered manifest contains a `ClusterRoleBinding` referencing `dynatrace-kubernetes-monitoring-sensitive`
