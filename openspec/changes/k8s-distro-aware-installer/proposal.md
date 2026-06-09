# Proposal

## Why

`dtwiz install kubernetes` applies a single generic DynaKube manifest regardless of which Kubernetes distribution is running. This causes silent failures on IKS and TKGI (wrong kubelet path), rejected pods on OpenShift (missing privileged annotation needed for SCC), and broken CSI injection on EKS Bottlerocket (immutable rootfs requires read-only volume mode). The installer must detect the distribution and generate a manifest tailored to its constraints.

## What Changes

- `DetectK8sDistribution()` extended with 5 missing distributions: GKE Autopilot, EKS Bottlerocket, IKS, RKE, TKGI — including sub-variant probing via live kubectl calls (node osImage, namespace active phase)
- Detection order enforced: parent distro confirmed first, sub-variant probed only when parent matches (GKE confirmed → Autopilot probe; EKS confirmed → Bottlerocket probe)
- Sub-variant classification extracted into pure `ClassifyK8sSubVariant(distro, output string, err error) string` for unit-testability without subprocess invocation
- `InstallKubernetes()` accepts `clusterName` and `distro` parameters; callers pass both from the already-detected `KubernetesInfo` to avoid redundant kubectl calls; empty `clusterName` falls back to internal derivation
- `dynakubeTemplateData` gains four new fields: `EnableKSPM`, `PrivilegedAnnotation`, `ReadOnlyVolume`, `KubeletPath`
- `dynakube.tmpl` gains conditional blocks driven by the new fields: KSPM section, per-DynaKube annotations, kubelet path override
- Missing `ClusterRoleBinding` added to the template

## Capabilities

### New Capabilities

- `k8s-distribution-detection`: Detecting GKE Autopilot, EKS Bottlerocket, IKS, RKE, and TKGI from live cluster signals (kubectl probes for node osImage and namespace existence), with correct sub-variant ordering
- `k8s-distro-aware-manifest`: Generating a DynaKube manifest conditioned on the detected distribution — KSPM block, privileged/readonly annotations, kubeletPath override

### Modified Capabilities

- `status-command-structure`: Distribution field in `KubernetesInfo` now populated for 5 additional distros; no requirement change, implementation only

## Impact

- `pkg/analyzer/detect_kubernetes.go` — extended with new distros, kubectl probes, and `ClassifyK8sSubVariant`
- `pkg/installer/kubernetes.go` — `clusterName` + `distro` params, new mapping function
- `pkg/installer/dynakube.tmpl` — conditional blocks added
- `cmd/install.go`, `cmd/setup.go` — pass detected `Cluster` and `Distribution` from `KubernetesInfo`
- No external API changes, no new flags, no breaking changes to existing behavior on already-supported distros
