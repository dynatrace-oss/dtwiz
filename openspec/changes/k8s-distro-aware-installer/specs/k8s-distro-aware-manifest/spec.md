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

### Requirement: Bottlerocket manifests carry readonly-volume annotation on both DynaKubes

The system SHALL add `feature.dynatrace.com/injection-readonly-volume: "true"` to the metadata annotations of both DynaKube objects when the distribution is EKS-Bottlerocket.

#### Scenario: Annotation present on both DynaKubes

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is `"EKS-Bottlerocket"`
- **THEN** both DynaKube objects in the rendered manifest contain `feature.dynatrace.com/injection-readonly-volume: "true"`

#### Scenario: Annotation absent on non-Bottlerocket distros

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** distro is any value other than `"EKS-Bottlerocket"`
- **THEN** neither DynaKube contains `injection-readonly-volume`

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
- **WHEN** distro is GKE, GKE-Autopilot, EKS, AKS, OpenShift, RKE, or generic kubernetes
- **THEN** rendered manifest does NOT contain `kubeletPath`

### Requirement: ClusterRoleBinding included in all rendered manifests

The system SHALL include a `ClusterRoleBinding` for `dynatrace-kubernetes-monitoring-sensitive` in every rendered manifest, binding to the `dynatrace-activegate` service account.

#### Scenario: ClusterRoleBinding present

- **GIVEN** `InstallKubernetes()` is called
- **WHEN** any distro string is passed
- **THEN** rendered manifest contains a `ClusterRoleBinding` referencing `dynatrace-kubernetes-monitoring-sensitive`
