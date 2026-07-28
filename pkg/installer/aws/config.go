// Package aws implements the Dynatrace AWS CloudFormation integration installer.
package aws

// awsStackConfig holds all values required to render aws.tmpl and drive the
// CloudFormation deployment.
type awsStackConfig struct {
	// StackName is the CloudFormation stack name (unique per account+region).
	StackName string

	// DynatraceURL is the full Dynatrace Platform environment URL.
	DynatraceURL string

	// SettingsToken is a platform token (dt0s16.*) with scopes:
	// settings:objects:write, extensions:configurations:write/read.
	SettingsToken string

	// IngestToken is a platform token (dt0s16.*) with scopes:
	// data-acquisition:logs:ingest, data-acquisition:events:ingest.
	IngestToken string

	// MonitoringConfigID is the UUID of the Dynatrace monitoring configuration.
	MonitoringConfigID string

	// LogsEnabled controls whether the log-forwarder resources are deployed.
	LogsEnabled string // "TRUE" | "FALSE"

	// LogsRegions is a comma-separated list of AWS regions for log ingestion.
	LogsRegions string

	// EventsEnabled controls whether the event-forwarder resources are deployed.
	EventsEnabled string // "TRUE" | "FALSE"

	// EventsRegions is a comma-separated list of AWS regions for event ingestion.
	EventsRegions string

	// EventBridgeBusName is the EventBridge bus to consume events from.
	EventBridgeBusName string

	// EventSources is a comma-separated list of event sources to forward.
	EventSources string

	// UseCMK controls whether a Customer Managed Key is created for encryption.
	UseCMK string // "TRUE" | "FALSE"
}

// awsTemplateURL points to the latest published Dynatrace CloudFormation
// template. Using the "latest" channel avoids 403s when a pinned version is
// removed from S3 and matches dtctl's behaviour.
const awsTemplateURL = "https://dynatrace-data-acquisition.s3.amazonaws.com/aws/deployment/cfn/latest/da-aws-activation.yaml"

// defaultFeatureSets is the standard set of AWS feature sets forwarded to Dynatrace.
var defaultFeatureSets = []string{
	"ApiGateway_essential", "ApplicationELB_essential", "AutoScaling_essential",
	"CloudFront_essential", "DynamoDB_essential", "EBS_essential", "EC2_essential",
	"ECR_essential", "ECS_ContainerInsights_essential", "ECS_essential", "EFS_essential",
	"ELB_essential", "ElastiCache_essential", "Firehose_essential", "Lambda_essential",
	"NATGateway_essential", "NetworkELB_essential", "PrivateLinkEndpoints_essential",
	"PrivateLinkServices_essential", "RDS_essential", "Route53_essential", "S3_essential",
	"SNS_essential", "SQS_essential",
}
