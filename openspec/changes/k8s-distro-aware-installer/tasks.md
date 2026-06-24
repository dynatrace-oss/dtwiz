# Tasks

## 1. Extend Distribution Detection

- [x] 1.1 Add IKS detection to `DetectK8sDistribution()` in `pkg/analyzer/detect_kubernetes.go` — match server URL containing `.containers.cloud.ibm.com`, return `"IKS"`
- [x] 1.2 Add RKE detection to `DetectK8sDistribution()` — match gitVersion containing `+rke2` (RKE2, not RKE1), return `"RKE"`; check before generic `"kubernetes"` fallback
- [x] 1.3 Add `ProbeK8sSubVariant(distro string) string` and `ClassifyK8sSubVariant(distro, output string, err error) string` in `pkg/analyzer/detect_kubernetes.go` — `ProbeK8sSubVariant` runs kubectl and delegates classification to the pure `ClassifyK8sSubVariant`; on error `ClassifyK8sSubVariant` returns parent distro unchanged
- [x] 1.4 Implement GKE Autopilot probe inside `ProbeK8sSubVariant`: run `kubectl get nodes -o jsonpath={.items[*].metadata.name}` with 5s timeout; return `"GKE-Autopilot"` if any node name has prefix `gk3-`, else return `"GKE"`. Note: `autopilot.gke.io` namespace annotation is absent on current Autopilot clusters; `cloud.google.com/gke-provisioning` node label is unreliable — node name prefix is the correct signal per GKE docs
- [x] 1.5 Implement EKS Bottlerocket probe inside `ProbeK8sSubVariant`: run `kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.osImage}` with 5s timeout; return `"EKS-Bottlerocket"` if output contains "Bottlerocket" (case-insensitive), else return `"EKS"`
- [x] 1.6 Implement TKGI fallback probe inside `ProbeK8sSubVariant`: when parent is `"kubernetes"`, run `kubectl get namespace pks-system --ignore-not-found -o jsonpath={.status.phase}` with 5s timeout; return `"TKGI"` only if output is `"Active"`
- [x] 1.7 Call `ProbeK8sSubVariant` from `detectKubernetes()` in `pkg/analyzer/detect_kubernetes.go` — after `DetectK8sDistribution()` returns, pass result through probe, store final value in `info.Distribution`

## 2. Detection Unit Tests

- [x] 2.1 Add table rows to `TestDetectK8sDistribution` in `pkg/analyzer/analyzer_test.go` for IKS (server URL signal) and RKE (gitVersion `+rke2` signal)
- [x] 2.2 Add `TestProbeK8sSubVariant` in `pkg/analyzer/analyzer_test.go` using fake kubectl (PATH injection via `t.TempDir`): assert GKE→GKE-Autopilot when annotation present, GKE→GKE when absent, EKS→EKS-Bottlerocket when osImage matches, EKS→EKS when not, TKGI probe triggers on unknown distro, all probes return parent on kubectl error
- [x] 2.3 Run `make test` and `make lint` — all pass

## 3. Wire Distro into Installer Signature

- [x] 3.1 Add `clusterName string` and `distro string` parameters to `InstallKubernetes()` in `pkg/installer/kubernetes.go` — `clusterName` replaces `name`, position before `distro` and `dryRun`; empty `clusterName` falls back to internal `fetchClusterName()`
- [x] 3.2 Update `installKubernetesCmd` in `cmd/install.go` and `setup.go` to call `analyzer.DetectKubernetes()` (or read from pre-run `AnalyzeSystem()` result), extract both `Cluster` and `Distribution`, pass both to `InstallKubernetes()` to avoid redundant kubectl calls
- [x] 3.3 Update dry-run output in `InstallKubernetes()` to print detected distro: `fmt.Printf("  Distribution: %s\n", distro)`
- [x] 3.4 Run `make build` — confirm compiles; run `make test` — confirm no regressions

## 4. Distro-to-Template Mapping

- [x] 4.1 Add fields to `dynakubeTemplateData` struct in `pkg/installer/kubernetes.go`: `EnableKSPM bool`, `PrivilegedAnnotation bool`, `ReadOnlyVolume bool`, `KubeletPath string`
- [x] 4.2 Implement `distroTemplateData(base dynakubeTemplateData, distro string) dynakubeTemplateData` in `pkg/installer/kubernetes.go` — set fields per distro: KSPM true for EKS/AKS/kubernetes/minikube/kind/k3s/empty (minikube, kind, and k3s use the same config as the generic "other" YAML — standard Linux paths, no platform-managed posture scanning); `PrivilegedAnnotation` true for OpenShift; `ReadOnlyVolume` true for EKS-Bottlerocket; `KubeletPath` `/var/data/kubelet` for IKS, `/var/vcap/data/kubelet` for TKGI; RKE/GKE/GKE-Autopilot/EKS-Bottlerocket/IKS/TKGI/OpenShift map to no KSPM unless noted above
- [x] 4.3 Call `distroTemplateData()` in `InstallKubernetes()` before passing data to `renderDynakubeTemplate()`

## 5. Update DynaKube Template

- [x] 5.1 Wrap KSPM block in `dynakube.tmpl` with `{{if .EnableKSPM}} ... {{end}}` — covers both `kspm.mappedHostPaths` and `templates.kspmNodeConfigurationCollector` blocks
- [x] 5.2 Add conditional annotation block to DynaKube #1 (monitoring) metadata in `dynakube.tmpl`: `{{if .PrivilegedAnnotation}}annotations:\n  feature.dynatrace.com/oneagent-privileged: "true"{{end}}` and equivalent for `ReadOnlyVolume`
- [x] 5.3 Add identical conditional annotation block to DynaKube #2 (agents) metadata in `dynakube.tmpl`
- [x] 5.4 Add conditional kubelet path field to both DynaKube specs in `dynakube.tmpl`: `{{if .KubeletPath}}  kubeletPath: {{.KubeletPath}}{{end}}`
- [x] 5.5 Add `ClusterRoleBinding` resource to `dynakube.tmpl` — bind `dynatrace-kubernetes-monitoring-sensitive` ClusterRole to `dynatrace-activegate` ServiceAccount in `dynatrace` namespace

## 5b. Template Structure Alignment

- [x] 5b.1 Move EEC, OTel collector, and log module `templates` entries from DynaKube #1 to DynaKube #2 in `dynakube.tmpl`; move `telemetryIngest`, `logMonitoring: {}`, and `extensions.prometheus: {}` from DynaKube #1 to DynaKube #2; DynaKube #1 `templates` block kept only for the conditional `kspmNodeConfigurationCollector` (absent entirely when KSPM disabled)

## 5c. GKE Autopilot Helm + Manifest Fixes

- [x] 5c.1 Add `disableCSI bool` parameter to `helmOperatorArgs()` and `helmOperatorUpgradeArgs()`; when true, append `--set csidriver.enabled=false` to the arg slice
- [x] 5c.2 In `InstallKubernetes()`, set `disableCSI := distro == "GKE-Autopilot"` and pass it to both Helm arg builders and their post-install-helm re-detection paths

## 6. Manifest Assertion Tests

- [x] 6.1 Create `pkg/installer/kubernetes_test.go` with `TestRenderDynakubeTemplate_Distros` table test — one row per distro string: `GKE`, `GKE-Autopilot`, `EKS`, `EKS-Bottlerocket`, `AKS`, `IKS`, `OpenShift`, `RKE`, `TKGI`, `kubernetes`, `minikube`, `kind`, `k3s`
- [x] 6.2 For each row assert specific substrings present or absent in rendered YAML: `mappedHostPaths` present/absent per KSPM expectation, correct annotation key present/absent, `kubeletPath` value present/absent with correct path, `ClusterRoleBinding` always present
- [x] 6.3 Run `make test` and `make lint` — all pass
