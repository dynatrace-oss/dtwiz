# Spec: OTel Cloud Entity Correlation

## NEW Requirements

### Requirement: Inject cloud correlation attribute into host metrics

The system SHALL set a Dynatrace cloud-correlation attribute on all host metrics when the collector is installed on a cloud VM, enabling the `OTEL_HOST runs_on <cloud entity>` Smartscape edge.

#### Scenario: Install on AWS EC2

- **GIVEN** dtwiz is generating an OTel Collector config on an AWS EC2 instance
- **WHEN** the install runs
- **THEN** the generated config includes the `ec2` resource detector alongside `system`
- **AND** includes a transform processor that sets `aws.arn` on every host metric

#### Scenario: Install on Azure VM

- **GIVEN** dtwiz is generating an OTel Collector config on an Azure Virtual Machine
- **WHEN** the install runs
- **THEN** the generated config includes the `azure` resource detector alongside `system`
- **AND** includes a transform processor that sets `azure.resource.id` (lowercase) on every host metric

#### Scenario: Install on GCP Compute Engine

- **GIVEN** dtwiz is generating an OTel Collector config on a GCP Compute Engine instance
- **WHEN** the install runs
- **THEN** the generated config includes the `gcp` resource detector alongside `system` with `gcp.gce.instance.name` enabled
- **AND** includes a transform processor that sets `gcp.resource.name` on every host metric

#### Scenario: Install on non-cloud machine

- **GIVEN** dtwiz is generating an OTel Collector config on a machine that is not a cloud VM
- **WHEN** the install runs
- **THEN** the generated config uses only the `system` detector
- **AND** no `transform/dt-cloud-correlation` processor is present

#### Scenario: Cloud metadata unavailable

- **GIVEN** the machine is a cloud VM but IMDS is unreachable (network policy, timeout)
- **WHEN** the install runs
- **THEN** dtwiz treats the machine as non-cloud and generates a config without cloud correlation
- **AND** the collector starts and exports metrics without blocking
