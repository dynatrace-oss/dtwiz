# Tasks

## 1. Extend Distribution Detection

- [ ] 1.1 Add IKS detection to `DetectK8sDistribution()` in `pkg/analyzer/detect_kubernetes.go` — match server URL containing `.containers.cloud.ibm.com`, return `"IKS"`
- [ ] 1.2 Add RKE detection to `DetectK8sDistribution()` — match gitVersion containing `+rke2` (RKE2, not RKE1), return `"RKE"`; check before generic `"kubernetes"` fallback
- [ ] 1.3 Add `ProbeK8sSubVariant(distro string) string` function in `pkg/analyzer/detect_kubernetes.go` — accepts parent distro, runs conditional kubectl probes, returns refined distro or parent unchanged
- [ ] 1.4 Implement GKE Autopilot probe inside `ProbeK8sSubVariant`: run `kubectl get namespace kube-system -o jsonpath={.metadata.annotations}` with 5s timeout; return `"GKE-Autopilot"` if output contains `autopilot.gke.io`, else return `"GKE"`
- [ ] 1.5 Implement EKS Bottlerocket probe inside `ProbeK8sSubVariant`: run `kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.osImage}` with 5s timeout; return `"EKS-Bottlerocket"` if output contains "Bottlerocket" (case-insensitive), else return `"EKS"`
- [ ] 1.6 Implement TKGI fallback probe inside `ProbeK8sSubVariant`: when parent is `"kubernetes"`, run `kubectl get namespace pks-system --ignore-not-found` with 5s timeout; return `"TKGI"` if output is non-empty
- [ ] 1.7 Call `ProbeK8sSubVariant` from `detectKubernetes()` in `pkg/analyzer/detect_kubernetes.go` — after `DetectK8sDistribution()` returns, pass result through probe, store final value in `info.Distribution`

## 2. Detection Unit Tests

- [ ] 2.1 Add table rows to `TestDetectK8sDistribution` in `pkg/analyzer/analyzer_test.go` for IKS (server URL signal) and RKE (gitVersion `+rke2` signal)
- [ ] 2.2 Add `TestProbeK8sSubVariant` in `pkg/analyzer/analyzer_test.go` using fake kubectl (PATH injection via `t.TempDir`): assert GKE→GKE-Autopilot when annotation present, GKE→GKE when absent, EKS→EKS-Bottlerocket when osImage matches, EKS→EKS when not, TKGI probe triggers on unknown distro, all probes return parent on kubectl error
- [ ] 2.3 Run `make test` and `make lint` — all pass

## 3. Wire Distro into Installer Signature

- [ ] 3.1 Add `distro string` parameter to `InstallKubernetes()` in `pkg/installer/kubernetes.go` — position after `name`, before `dryRun`
- [ ] 3.2 Update `installKubernetesCmd` in `cmd/install.go` to call `analyzer.DetectKubernetes()` (or read from a pre-run `AnalyzeSystem()` result), extract `Distribution`, pass it to `InstallKubernetes()`
- [ ] 3.3 Update dry-run output in `InstallKubernetes()` to print detected distro: `fmt.Printf("  Distribution: %s\n", distro)`
- [ ] 3.4 Run `make build` — confirm compiles; run `make test` — confirm no regressions

## 4. Distro-to-Template Mapping

- [ ] 4.1 Add fields to `dynakubeTemplateData` struct in `pkg/installer/kubernetes.go`: `EnableKSPM bool`, `PrivilegedAnnotation bool`, `ReadOnlyVolume bool`, `KubeletPath string`
- [ ] 4.2 Implement `distroTemplateData(base dynakubeTemplateData, distro string) dynakubeTemplateData` in `pkg/installer/kubernetes.go` — set fields per distro: KSPM true for EKS/AKS/kubernetes/empty; `PrivilegedAnnotation` true for OpenShift; `ReadOnlyVolume` true for EKS-Bottlerocket; `KubeletPath` `/var/data/kubelet` for IKS, `/var/vcap/data/kubelet` for TKGI; RKE maps to no KSPM, no annotations, no kubeletPath
- [ ] 4.3 Call `distroTemplateData()` in `InstallKubernetes()` before passing data to `renderDynakubeTemplate()`

## 5. Update DynaKube Template

- [ ] 5.1 Wrap KSPM block in `dynakube.tmpl` with `{{if .EnableKSPM}} ... {{end}}` — covers both `kspm.mappedHostPaths` and `templates.kspmNodeConfigurationCollector` blocks
- [ ] 5.2 Add conditional annotation block to DynaKube #1 (monitoring) metadata in `dynakube.tmpl`: `{{if .PrivilegedAnnotation}}annotations:\n  feature.dynatrace.com/oneagent-privileged: "true"{{end}}` and equivalent for `ReadOnlyVolume`
- [ ] 5.3 Add identical conditional annotation block to DynaKube #2 (agents) metadata in `dynakube.tmpl`
- [ ] 5.4 Add conditional kubelet path field to both DynaKube specs in `dynakube.tmpl`: `{{if .KubeletPath}}  kubeletPath: {{.KubeletPath}}{{end}}`
- [ ] 5.5 Add `ClusterRoleBinding` resource to `dynakube.tmpl` — bind `dynatrace-kubernetes-monitoring-sensitive` ClusterRole to `dynatrace-activegate` ServiceAccount in `dynatrace` namespace

## 6. Manifest Assertion Tests

- [ ] 6.1 Create `pkg/installer/kubernetes_test.go` with `TestRenderDynakubeTemplate_Distros` table test — one row per distro string: `GKE`, `GKE-Autopilot`, `EKS`, `EKS-Bottlerocket`, `AKS`, `IKS`, `OpenShift`, `RKE`, `TKGI`, `kubernetes`
- [ ] 6.2 For each row assert specific substrings present or absent in rendered YAML: `mappedHostPaths` present/absent per KSPM expectation, correct annotation key present/absent, `kubeletPath` value present/absent with correct path, `ClusterRoleBinding` always present
- [ ] 6.3 Run `make test` and `make lint` — all pass
