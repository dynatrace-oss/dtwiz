# Tasks

## 1. Display helpers

- [x] 1.1 Add `pkg/display/step.go` with `RunStep`, `Step.Detail`, `StepDone`, `StepWarn` and `Detail`.
- [x] 1.2 Mark reported errors and silence cobra's error printing in favour of one handler in `cmd/root.go`.
- [x] 1.3 Add tests for plain, animated, failure, tail and concurrent printing.

## 2. Install flows

- [x] 2.1 OneAgent: steps for connectivity, download, signature and install; prime sudo.
- [x] 2.2 OTel Collector: steps, compact summary, compact tenant-setup preview and apply summary.
- [x] 2.3 Kubernetes: steps and compact preview.
- [x] 2.4 AWS (including Lambda), Azure and GCP: steps and compact preview.
- [x] 2.5 OTel runtime executors (Python, Node.js, Java): steps instead of inline status lines, and per-service lines for started services.
- [x] 2.6 Uninstall commands: kubernetes, aws, aws-lambda, azure, gcp, otel, oneagent, self (Unix).
- [x] 2.7 Update commands (azure, gcp, otel), `install demo` and `install docker`.
- [x] 2.8 Keep the spinner on with `-v`/`--debug`: spinner-aware logger and subprocess writers.
- [x] 2.9 Print the install header once (`display.InstallHeader`, set by `dtwiz setup`).
- [x] 2.10 `display.RunQuiet` for silent phases (tenant setup, lookups, scans, Helm install).
- [x] 2.11 Standalone `install otel-python`, `otel-node`, `otel-java`: compact preview, header and `Apply?`; demo skips the nested preview and prompt.

## 3. Tests and docs

- [x] 3.1 Update tests that asserted on commands, manifests or config in the default preview.
- [x] 3.2 Add a CHANGELOG entry.
