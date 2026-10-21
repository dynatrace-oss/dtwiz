# Tasks

## 1. Display helpers

- [x] 1.1 Add `pkg/display/step.go` with `RunStep`, `Step.Detail`, `StepDone`, `StepWarn` and `Detail`.
- [x] 1.2 Mark reported errors and silence cobra's error printing in favour of one handler in `cmd/root.go`.
- [x] 1.3 Add tests for plain, animated, failure, tail and concurrent printing.

## 2. Install flows

- [x] 2.1 OneAgent: steps for connectivity, download, signature and install; prime sudo.
- [x] 2.2 OTel Collector: steps, compact summary, compact tenant-setup preview and apply summary.
- [x] 2.3 Kubernetes: steps and compact preview.
- [x] 2.4 AWS, Azure and GCP: steps and compact preview.

## 3. Tests and docs

- [x] 3.1 Update tests that asserted on commands, manifests or config in the default preview.
- [x] 3.2 Add a CHANGELOG entry.
