# K8s Distribution Detection

## Purpose

Define how `dtwiz analyze` detects the Kubernetes distribution (EKS, GKE, AKS, K3s, etc.) and reports it in system analysis.

## Requirements

### Requirement: Detect GKE Autopilot as distinct from GKE Standard

The system SHALL identify a GKE Autopilot cluster by checking node names after the parent GKE distro is confirmed. GKE Autopilot nodes use the `gk3-` name prefix; GKE Standard nodes use `gke-`. The returned distribution string SHALL be `"GKE-Autopilot"`. Node name prefix is the officially documented detection signal.

#### Scenario: Autopilot cluster detected

- **GIVEN** `DetectK8sDistribution` has returned `"GKE"`
- **WHEN** `kubectl get nodes -o jsonpath={.items[*].metadata.name}` returns at least one node name starting with `gk3-`
- **THEN** `ProbeK8sSubVariant` returns `"GKE-Autopilot"`

#### Scenario: Standard GKE cluster unchanged

- **GIVEN** `DetectK8sDistribution` has returned `"GKE"`
- **WHEN** all node names start with `gke-` (no `gk3-` prefix found)
- **THEN** `ProbeK8sSubVariant` returns `"GKE"`

#### Scenario: Probe fails gracefully

- **GIVEN** `DetectK8sDistribution` has returned `"GKE"`
- **WHEN** the `kubectl get nodes` call returns an error or times out
- **THEN** `ProbeK8sSubVariant` returns the parent distro unchanged (`"GKE"`)

### Requirement: Detect EKS Bottlerocket as distinct from EKS Standard

The system SHALL identify an EKS Bottlerocket cluster by checking node `osImage` fields after the parent EKS distro is confirmed. The returned distribution string SHALL be `"EKS-Bottlerocket"`.

#### Scenario: Bottlerocket nodes detected

- **GIVEN** `DetectK8sDistribution` has returned `"EKS"`
- **WHEN** `kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.osImage}` output contains "Bottlerocket"
- **THEN** `ProbeK8sSubVariant` returns `"EKS-Bottlerocket"`

#### Scenario: Standard EKS nodes unchanged

- **GIVEN** `DetectK8sDistribution` has returned `"EKS"`
- **WHEN** node osImage output does NOT contain "Bottlerocket"
- **THEN** `ProbeK8sSubVariant` returns `"EKS"`

#### Scenario: Probe fails gracefully

- **GIVEN** `DetectK8sDistribution` has returned `"EKS"`
- **WHEN** the node osImage kubectl call returns an error or times out
- **THEN** `ProbeK8sSubVariant` returns `"EKS"`

### Requirement: Detect IKS by server URL

The system SHALL identify an IBM Kubernetes Service cluster when the API server URL contains `.containers.cloud.ibm.com`. The returned distribution string SHALL be `"IKS"`.

#### Scenario: IKS server URL matched

- **GIVEN** the kubeconfig current context points to a cluster
- **WHEN** the cluster server URL contains `.containers.cloud.ibm.com`
- **THEN** `DetectK8sDistribution` returns `"IKS"`

### Requirement: Detect RKE by server version

The system SHALL identify an RKE cluster when the server gitVersion contains `+rke2` (Rancher Kubernetes Engine 2, not RKE1). The returned distribution string SHALL be `"RKE"`.

#### Scenario: RKE gitVersion matched

- **GIVEN** the cluster is reachable and `kubectl version` succeeds
- **WHEN** the server gitVersion string contains `+rke2`
- **THEN** `DetectK8sDistribution` returns `"RKE"`

### Requirement: Detect TKGI by namespace probe

The system SHALL identify a TKGI cluster by checking whether the `pks-system` namespace exists and is `Active` after no other distro is matched. The returned distribution string SHALL be `"TKGI"`.

#### Scenario: TKGI namespace active

- **GIVEN** no other distro signal matches the cluster
- **WHEN** `kubectl get namespace pks-system --ignore-not-found -o jsonpath={.status.phase}` returns `"Active"`
- **THEN** `ClassifyK8sSubVariant` returns `"TKGI"`

#### Scenario: Namespace absent

- **GIVEN** no other distro signal matches the cluster
- **WHEN** `pks-system` namespace does not exist (empty output from `--ignore-not-found`)
- **THEN** detection falls through to `"kubernetes"` default

#### Scenario: Namespace exists but inactive

- **GIVEN** no other distro signal matches the cluster
- **WHEN** `pks-system` namespace exists but `.status.phase` is not `"Active"` (e.g. `"Terminating"`)
- **THEN** detection falls through to `"kubernetes"` default

### Requirement: Sub-variant classification logic is pure and separately testable

The system SHALL implement sub-variant classification as `ClassifyK8sSubVariant(distro, output string, err error) string` — a pure function that takes the parent distro, kubectl output, and probe error, and returns the refined distro. `ProbeK8sSubVariant` SHALL invoke the kubectl probe and delegate the classification decision to `ClassifyK8sSubVariant`.

#### Scenario: Error propagates to parent distro

- **GIVEN** any parent distro
- **WHEN** `ClassifyK8sSubVariant` is called with a non-nil `err`
- **THEN** it returns the parent distro unchanged, regardless of `output`

### Requirement: Sub-variant probes run only when parent distro matches

The system SHALL invoke sub-variant kubectl probes only after the parent distribution is confirmed, to avoid unnecessary API calls on non-matching clusters.

#### Scenario: Autopilot probe skipped on non-GKE cluster

- **GIVEN** the cluster has been analyzed
- **WHEN** `DetectK8sDistribution` returns any value other than `"GKE"`
- **THEN** the `kube-system` annotation probe is NOT executed

#### Scenario: Bottlerocket probe skipped on non-EKS cluster

- **GIVEN** the cluster has been analyzed
- **WHEN** `DetectK8sDistribution` returns any value other than `"EKS"`
- **THEN** the node osImage probe is NOT executed
