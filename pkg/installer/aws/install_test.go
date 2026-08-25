package aws

import (
	"reflect"
	"strings"
	"testing"
)

func TestMaskTokenArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		tokens []string
		want   []string
	}{
		{
			name:   "token value is fully masked, not partially revealed",
			args:   []string{"pDtApiToken=dt0s16.abcdefghijklmnop"},
			tokens: []string{"dt0s16.abcdefghijklmnop"},
			want:   []string{"pDtApiToken=***"},
		},
		{
			name:   "short token value is still fully masked",
			args:   []string{"pDtIngestToken=short"},
			tokens: []string{"short"},
			want:   []string{"pDtIngestToken=***"},
		},
		{
			name:   "multiple distinct tokens are each masked",
			args:   []string{"pDtApiToken=tokenA pDtIngestToken=tokenB"},
			tokens: []string{"tokenA", "tokenB"},
			want:   []string{"pDtApiToken=*** pDtIngestToken=***"},
		},
		{
			name:   "args without a matching token are untouched",
			args:   []string{"--stack-name", "dynatrace-data-acquisition"},
			tokens: []string{"tokenA"},
			want:   []string{"--stack-name", "dynatrace-data-acquisition"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := maskTokenArgs(tt.args, tt.tokens...)
			if len(got) != len(tt.want) {
				t.Fatalf("maskTokenArgs(%v, %v) = %v, want %v", tt.args, tt.tokens, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("maskTokenArgs(%v, %v)[%d] = %q, want %q", tt.args, tt.tokens, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildDeployArgs(t *testing.T) {
	t.Parallel()

	cfg := awsStackConfig{
		StackName:          "dynatrace-data-acquisition",
		DynatraceURL:       "https://abc123.apps.dynatrace.com",
		SettingsToken:      "settings-token",
		IngestToken:        "ingest-token",
		MonitoringConfigID: "monitoring-config-id",
		LogsEnabled:        "TRUE",
		LogsRegions:        "eu-west-1,us-east-1",
		EventsEnabled:      "FALSE",
		EventsRegions:      "ap-southeast-2,eu-central-1",
		EventBridgeBusName: "custom-bus",
		EventSources:       "aws.health,aws.ec2",
		UseCMK:             "FALSE",
	}

	got := buildDeployArgs(cfg, "/tmp/da-aws-activation.yaml")
	want := []string{
		"cloudformation", "deploy",
		"--stack-name", "dynatrace-data-acquisition",
		"--template-file", "/tmp/da-aws-activation.yaml",
		"--capabilities", "CAPABILITY_NAMED_IAM",
		"--parameter-overrides",
		"pDynatraceUrl=https://abc123.apps.dynatrace.com",
		"pDtApiToken=settings-token",
		"pDtIngestToken=ingest-token",
		"pMonitoringConfigId=monitoring-config-id",
		"pDtLogsIngestEnabled=TRUE",
		"pDtLogsIngestRegions=eu-west-1,us-east-1",
		"pDtEventsIngestEnabled=FALSE",
		"pDtEventsIngestRegions=ap-southeast-2,eu-central-1",
		"pEventBridgeBusName=custom-bus",
		"pEventSources=aws.health,aws.ec2",
		"pUseCMK=FALSE",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildDeployArgs() = %#v, want %#v", got, want)
	}
}

func TestFormatDeployCmd(t *testing.T) {
	t.Parallel()

	got := formatDeployCmd([]string{
		"cloudformation", "deploy",
		"--stack-name", "dynatrace-data-acquisition",
		"--template-file", "/tmp/da-aws-activation.yaml",
		"--capabilities", "CAPABILITY_NAMED_IAM",
		"--parameter-overrides",
		"pDynatraceUrl=https://abc123.apps.dynatrace.com",
		"pDtLogsIngestRegions=eu-west-1,us-east-1",
	})

	wantContains := []string{
		"cloudformation deploy \\",
		"\n        --stack-name dynatrace-data-acquisition \\",
		"\n        --template-file /tmp/da-aws-activation.yaml \\",
		"\n        --capabilities CAPABILITY_NAMED_IAM \\",
		"\n        --parameter-overrides \\",
		"\n            pDynatraceUrl=https://abc123.apps.dynatrace.com \\",
		"\n            pDtLogsIngestRegions=eu-west-1,us-east-1",
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Fatalf("formatDeployCmd() = %q, want substring %q", got, want)
		}
	}
}
