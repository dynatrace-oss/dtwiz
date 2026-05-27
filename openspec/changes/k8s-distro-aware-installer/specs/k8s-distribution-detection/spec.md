# K8s Distribution Detection

## ADDED Requirements

### Requirement: Detect GKE Autopilot as distinct from GKE Standard

The system SHALL identify a GKE Autopilot cluster by probing the `kube-system` namespace annotations for the `autopilot.gke.io` key after the parent GKE distro is confirmed. The returned distribution string SHALL be `"GKE-Autopilot"`.

#### Scenario: Autopilot cluster detected

- **WHEN** `DetectK8sDistribution` returns `"GKE"` AND `kubectl get namespace kube-system` annotations contain `autopilot.gke.io`
- **THEN** `ProbeK8sSubVariant` returns `"GKE-Autopilot"`

#### Scenario: Standard GKE cluster unchanged

- **WHEN** `DetectK8sDistribution` returns `"GKE"` AND `kube-system` annotations do NOT contain `autopilot.gke.io`
- **THEN** `ProbeK8sSubVariant` returns `"GKE"`

#### Scenario: Probe fails gracefully

- **WHEN** the `kubectl get namespace kube-system` call returns an error or times out
- **THEN** `ProbeK8sSubVariant` returns the parent distro unchanged (`"GKE"`)

### Requirement: Detect EKS Bottlerocket as distinct from EKS Standard

The system SHALL identify an EKS Bottlerocket cluster by checking node `osImage` fields after the parent EKS distro is confirmed. The returned distribution string SHALL be `"EKS-Bottlerocket"`.

#### Scenario: Bottlerocket nodes detected

- **WHEN** `DetectK8sDistribution` returns `"EKS"` AND `kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.osImage}` output contains "Bottlerocket"
- **THEN** `ProbeK8sSubVariant` returns `"EKS-Bottlerocket"`

#### Scenario: Standard EKS nodes unchanged

- **WHEN** `DetectK8sDistribution` returns `"EKS"` AND node osImage output does NOT contain "Bottlerocket"
- **THEN** `ProbeK8sSubVariant` returns `"EKS"`

#### Scenario: Probe fails gracefully

- **WHEN** the node osImage kubectl call returns an error or times out
- **THEN** `ProbeK8sSubVariant` returns `"EKS"`

### Requirement: Detect IKS by server URL

The system SHALL identify an IBM Kubernetes Service cluster when the API server URL contains `.containers.cloud.ibm.com`. The returned distribution string SHALL be `"IKS"`.

#### Scenario: IKS server URL matched

- **WHEN** the cluster server URL contains `.containers.cloud.ibm.com`
- **THEN** `DetectK8sDistribution` returns `"IKS"`

### Requirement: Detect RKE2 by server version

The system SHALL identify an RKE2 cluster when the server gitVersion contains `+rke2`. The returned distribution string SHALL be `"RKE2"`.

#### Scenario: RKE2 gitVersion matched

- **WHEN** the server gitVersion string contains `+rke2`
- **THEN** `DetectK8sDistribution` returns `"RKE2"`

### Requirement: Detect TKGI by namespace probe

The system SHALL identify a TKGI cluster by checking whether the `pks-system` namespace exists after no other distro is matched. The returned distribution string SHALL be `"TKGI"`.

#### Scenario: TKGI namespace found

- **WHEN** no other distro signal matches AND `kubectl get namespace pks-system --ignore-not-found` returns a non-empty result
- **THEN** `ProbeK8sSubVariant` (or a fallback probe) returns `"TKGI"`

#### Scenario: Namespace absent

- **WHEN** `pks-system` namespace does not exist
- **THEN** detection falls through to `"kubernetes"` default

### Requirement: Sub-variant probes run only when parent distro matches

The system SHALL invoke sub-variant kubectl probes only after the parent distribution is confirmed, to avoid unnecessary API calls on non-matching clusters.

#### Scenario: Autopilot probe skipped on non-GKE cluster

- **WHEN** `DetectK8sDistribution` returns any value other than `"GKE"`
- **THEN** the `kube-system` annotation probe is NOT executed

#### Scenario: Bottlerocket probe skipped on non-EKS cluster

- **WHEN** `DetectK8sDistribution` returns any value other than `"EKS"`
- **THEN** the node osImage probe is NOT executed
