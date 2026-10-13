# Proposal

## Why

The agents DynaKube (`{{ .ClusterName }}-agents`) previously rendered an `extensions.prometheus: {}` block in `pkg/installer/kubernetes/dynakube.tmpl`. This was removed from the template because the Dynatrace Operator does not require an explicit opt-in to enable Prometheus extension scraping on the agents DynaKube — enabling it unconditionally added an unused stanza to every generated manifest. The template and its test (`TestRenderDynakubeTemplate_AgentsDynaKubeStructure`) were already updated accordingly, but the standalone spec at `openspec/specs/k8s-distro-aware-manifest/spec.md` still documents `extensions`/`extensions.prometheus: {}` as a required part of DynaKube #2, leaving the spec out of sync with the shipped behavior.

## What Changes

- Update the "Templates block belongs only in DynaKube #2 (agents), not DynaKube #1 (monitoring)" requirement to drop the `extensions`/`extensions.prometheus: {}` references.
- Update the associated scenario ("DynaKube #2 carries all extension templates") to no longer assert `extensions.prometheus: {}` is present.

No code or test changes are included in this change — `pkg/installer/kubernetes/dynakube.tmpl` and `pkg/installer/kubernetes/install_test.go` were already updated in a prior commit. This change brings the spec documentation back in line with that shipped behavior.

## Capabilities

### Modified Capabilities

- `k8s-distro-aware-manifest`: remove the `extensions.prometheus: {}` requirement from the agents DynaKube template documentation.

## Impact

- Documentation-only change to `openspec/specs/k8s-distro-aware-manifest/spec.md`.
- No production code, template, or test changes (already shipped).
- No CLI, UX, or behavior changes.
