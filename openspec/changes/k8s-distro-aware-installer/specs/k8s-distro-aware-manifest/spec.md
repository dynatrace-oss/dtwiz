# K8s Distro-Aware Manifest

## ADDED Requirements

### Requirement: InstallKubernetes accepts distribution parameter

The system SHALL accept a `distro string` parameter in `InstallKubernetes()`. When empty, it SHALL default to `"kubernetes"` behavior.

#### Scenario: Distro wired from command

- **WHEN** `installKubernetesCmd` runs AND the analyzer has detected a Kubernetes cluster
- **THEN** the detected `Distribution` value from `KubernetesInfo` SHALL be passed to `InstallKubernetes()`

#### Scenario: Empty distro falls back to default

- **WHEN** `InstallKubernetes()` receives an empty `distro` string
- **THEN** the rendered manifest SHALL match the `"kubernetes"` (generic) distro behavior — KSPM enabled, no annotations, no kubeletPath override

### Requirement: KSPM block included only for standard Linux distributions

The system SHALL include the `kspm.mappedHostPaths` block and `kspmNodeConfigurationCollector` image reference in the rendered manifest only for distributions where standard Linux host paths are accessible.

#### Scenario: KSPM rendered for EKS

- **WHEN** distro is `"EKS"`
- **THEN** rendered manifest contains `mappedHostPaths` with all 7 paths and `kspmNodeConfigurationCollector` image ref

#### Scenario: KSPM rendered for AKS

- **WHEN** distro is `"AKS"`
- **THEN** rendered manifest contains `mappedHostPaths` block

#### Scenario: KSPM rendered for generic kubernetes

- **WHEN** distro is `"kubernetes"` or empty
- **THEN** rendered manifest contains `mappedHostPaths` block

#### Scenario: KSPM omitted for GKE

- **WHEN** distro is `"GKE"` or `"GKE-Autopilot"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for OpenShift

- **WHEN** distro is `"OpenShift"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for Bottlerocket

- **WHEN** distro is `"EKS-Bottlerocket"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for RKE2

- **WHEN** distro is `"RKE2"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for IKS

- **WHEN** distro is `"IKS"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

#### Scenario: KSPM omitted for TKGI

- **WHEN** distro is `"TKGI"`
- **THEN** rendered manifest does NOT contain `mappedHostPaths`

### Requirement: OpenShift manifests carry privileged annotation on both DynaKubes

The system SHALL add `feature.dynatrace.com/oneagent-privileged: "true"` to the metadata annotations of both DynaKube objects when the distribution is OpenShift.

#### Scenario: Annotation present on both DynaKubes

- **WHEN** distro is `"OpenShift"`
- **THEN** both the monitoring DynaKube and the agents DynaKube in the rendered manifest contain `feature.dynatrace.com/oneagent-privileged: "true"`

#### Scenario: Annotation absent on non-OpenShift distros

- **WHEN** distro is any value other than `"OpenShift"`
- **THEN** neither DynaKube in the rendered manifest contains `oneagent-privileged`

### Requirement: Bottlerocket manifests carry readonly-volume annotation on both DynaKubes

The system SHALL add `feature.dynatrace.com/injection-readonly-volume: "true"` to the metadata annotations of both DynaKube objects when the distribution is EKS-Bottlerocket.

#### Scenario: Annotation present on both DynaKubes

- **WHEN** distro is `"EKS-Bottlerocket"`
- **THEN** both DynaKube objects in the rendered manifest contain `feature.dynatrace.com/injection-readonly-volume: "true"`

#### Scenario: Annotation absent on non-Bottlerocket distros

- **WHEN** distro is any value other than `"EKS-Bottlerocket"`
- **THEN** neither DynaKube contains `injection-readonly-volume`

### Requirement: Non-standard kubelet paths set in manifest for IKS and TKGI

The system SHALL set the appropriate `kubeletPath` in the rendered DynaKube spec when the distribution uses a non-default kubelet data directory.

#### Scenario: IKS kubeletPath set

- **WHEN** distro is `"IKS"`
- **THEN** rendered manifest contains `kubeletPath: /var/data/kubelet`

#### Scenario: TKGI kubeletPath set

- **WHEN** distro is `"TKGI"`
- **THEN** rendered manifest contains `kubeletPath: /var/vcap/data/kubelet`

#### Scenario: kubeletPath absent for standard distros

- **WHEN** distro is GKE, EKS, AKS, OpenShift, RKE2, or generic kubernetes
- **THEN** rendered manifest does NOT contain `kubeletPath`

### Requirement: ClusterRoleBinding included in all rendered manifests

The system SHALL include a `ClusterRoleBinding` for `dynatrace-kubernetes-monitoring-sensitive` in every rendered manifest, binding to the `dynatrace-activegate` service account.

#### Scenario: ClusterRoleBinding present

- **WHEN** any distro string is passed to `InstallKubernetes()`
- **THEN** rendered manifest contains a `ClusterRoleBinding` referencing `dynatrace-kubernetes-monitoring-sensitive`
