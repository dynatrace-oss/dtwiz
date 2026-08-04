# Implementation Tasks

## GCP

- [x] 1.1 Replace `integrationName` constant in `pkg/installer/gcp/config.go` with `integrationPrefix` and `integrationNameForEnv(envURL)`.
- [x] 1.2 Derive integration name in `install.go` and `update.go`; add `name` parameter to `gcpResumableConnection` and `selectUpdatableConnection`.
- [x] 1.3 Use prefix matching in `dtapi.go`; update `connectionExistsWithClient` and `MonitoringConfigExists` to use `integrationPrefix`.
- [x] 1.4 Split service-account cleanup into current/legacy in `uninstall.go`; treat legacy failures as warnings.
- [x] 1.5 `go test ./pkg/installer/gcp/...` passes.

## Azure

- [x] 2.1 Replace `integrationName` constant in `pkg/installer/azure/config.go` with `integrationPrefix` and `integrationNameForEnv(envURL)`.
- [x] 2.2 Derive integration name in `install.go` and `update.go`; add `name` parameter to `selectUpdatableConnection`; run account lookup before DT discovery in update.
- [x] 2.3 Use prefix matching in `dtapi.go` and `extension_client.go`; update `connectionExistsWithClient` and `MonitoringConfigExists`.
- [x] 2.4 Split App Registration cleanup into current/legacy in `uninstall.go`; treat legacy failures as warnings.
- [x] 2.5 `go test ./pkg/installer/azure/... ./pkg/installer/...` passes.
