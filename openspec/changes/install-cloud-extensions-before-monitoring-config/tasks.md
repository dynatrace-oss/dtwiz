# Implementation Tasks

## 1. Shared Extension Activation

- [x] 1.1 Add an idempotent `InstallExtension` helper to `pkg/installer/extension_client.go` using the dtctl SDK `InstallFromHub` API.
- [x] 1.2 Add debug logs for extension activation attempt, already-installed handling, and successful activation.
- [x] 1.3 Add unit tests in `pkg/installer/extension_client_test.go` for successful activation and already-installed responses.

## 2. Azure Integration

- [x] 2.1 Add Azure extension name/version wiring in `pkg/installer/azure/dtapi.go` and expose activation through the package-local DT client interface.
- [x] 2.2 Call extension activation in `pkg/installer/azure/install.go` after connection finalization and before monitoring config creation.
- [x] 2.3 Call extension activation in `pkg/installer/azure/update.go` after confirmation and before monitoring config reconcile.
- [x] 2.4 Add Azure install/update tests proving activation occurs and activation failure prevents monitoring config mutation.

## 3. GCP Integration

- [x] 3.1 Add GCP extension name/version wiring in `pkg/installer/gcp/dtapi.go` and expose activation through the package-local DT client interface.
- [x] 3.2 Call extension activation in `pkg/installer/gcp/install.go` after connection finalization and before monitoring config creation.
- [x] 3.3 Call extension activation in `pkg/installer/gcp/update.go` after confirmation and before monitoring config reconcile.
- [x] 3.4 Add GCP install/update tests proving activation occurs and activation failure prevents monitoring config mutation.

## 4. Validation

- [x] 4.1 Run `go test ./pkg/installer ./pkg/installer/azure ./pkg/installer/gcp`.
- [x] 4.2 Run `openspec status --change install-cloud-extensions-before-monitoring-config` and confirm the change is apply-ready.
