# Design

## Context

`InstallKubernetes()` in `pkg/installer/kubernetes.go` renders a single `dynakube.tmpl` and applies it unconditionally. The analyzer already detects GKE, EKS, AKS, and OpenShift via `DetectK8sDistribution()`, but the detected value is never passed to the installer. Five distributions (GKE Autopilot, EKS Bottlerocket, IKS, RKE, TKGI) are not detected at all. The result: KSPM always enabled regardless of platform, no annotations applied, no kubelet path overrides — silent failures on IKS and TKGI, pod admission rejections on OpenShift, CSI write errors on Bottlerocket.

## Goals / Non-Goals

**Goals:**

- Wire detected distribution from analyzer into the installer
- Extend detection to cover all 10 distributions in `k8s-yamls/`
- Conditionalize the DynaKube manifest on distribution: KSPM block, privileged/readonly annotations, kubeletPath override
- All changes covered by unit + snapshot tests

**Non-Goals:**

- Changing the OneAgent mode (all distributions remain `applicationMonitoring`)
- Cloud-cluster integration tests (unit + fake-kubectl coverage only)
- Modifying the Helm install/upgrade logic
- Structural DynaKube template refactor beyond adding conditionals

## Decisions

### 1. Pass `distro string` into `InstallKubernetes()` rather than re-detecting inside it

The analyzer already runs before `installKubernetesCmd` executes. Re-detecting inside the installer would duplicate kubectl calls and add latency. The command layer fetches `SystemInfo`, reads `info.Kubernetes.Distribution`, and passes it down. If analyzer hasn't run (dry-run or future non-interactive path), the empty string falls through to the `"kubernetes"` default case in `distroTemplateData()`.

Alternative considered: detect inside `InstallKubernetes()`. Rejected — violates single-responsibility, adds ~3 kubectl calls to install path already covered by analyze.

### 2. New fields on `dynakubeTemplateData` over per-distro template files

10 separate template files would be harder to keep in sync and would duplicate ~90% of content. Instead, 4 boolean/string fields drive `{{if}}` blocks in a single template. The diff between distros is small (KSPM block, one annotation, one path field) — a single template with conditionals is the right granularity.

Alternative considered: per-distro YAML files embedded as separate `//go:embed` assets. Rejected — maintenance burden, no type safety on fields, harder to test rendering.

### 3. Sub-variant detection requires live kubectl probes beyond the existing 4-string signature

`DetectK8sDistribution(context, cluster, serverURL, serverVersion string)` is a pure function — fast, testable, no I/O. GKE Autopilot and EKS Bottlerocket cannot be distinguished from their parents using only these four strings. Two new probes are needed:

- Autopilot: `kubectl get namespace kube-system -o jsonpath={.metadata.annotations}` — check for `autopilot.gke.io`
- Bottlerocket: `kubectl get nodes -o jsonpath={.items[*].status.nodeInfo.osImage}` — check for "Bottlerocket"

These probes run only after the parent distro is confirmed (GKE or EKS), keeping the happy path fast. The pure `DetectK8sDistribution()` function remains unchanged for unit-testability; a new `ProbeK8sSubVariant(distro string) string` function wraps the kubectl calls.

### 4. Detection order: resolve sub-variants after parent match

`ProbeK8sSubVariant` is called after `DetectK8sDistribution` returns the parent distro. This keeps the existing pure function contract intact and avoids adding kubectl calls to every detection path.

## Risks / Trade-offs

`ProbeK8sSubVariant` kubectl calls add latency on GKE and EKS paths (~1–2s per probe with 5s timeout) → mitigated by running probes only when parent matches, with short timeouts and graceful fallback to parent distro on error.

TKGI detection via `pks-system` namespace may produce false positives on clusters that previously ran TKGI but were migrated → acceptable edge case; user can override with `--platform` flag (future).

`dynakube.tmpl` conditional blocks increase template complexity → mitigated by table-driven manifest assertion tests per distro.

## Open Questions

None blocking implementation.
