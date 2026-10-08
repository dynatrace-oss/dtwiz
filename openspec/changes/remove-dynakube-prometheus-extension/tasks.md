# Tasks

## 1. Spec Documentation

- [ ] 1.1 In `openspec/specs/k8s-distro-aware-manifest/spec.md`, update the "Templates block belongs only in DynaKube #2 (agents), not DynaKube #1 (monitoring)" requirement text to remove the `extensions` reference.
- [ ] 1.2 Update the "DynaKube #2 carries all extension templates" scenario to remove the `extensions.prometheus: {}` assertion.
- [ ] 1.3 Confirm no other spec or doc references `extensions.prometheus: {}` for the Kubernetes installer (`grep -rn "extensions.prometheus" openspec/ pkg/installer/kubernetes/`).

## 2. Verification

- [ ] 2.1 Run `go test ./pkg/installer/kubernetes/...` to confirm the already-updated template and tests still pass.
