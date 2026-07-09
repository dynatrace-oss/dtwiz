package installer

import (
	"fmt"
	"strings"
	"testing"
)

// nodesResp builds a dqlResponse for the smartscapeNodes query (type + count fields).
func nodesResp(rows ...struct {
	typeName string
	count    int
}) *dqlResponse {
	resp := &dqlResponse{}
	for _, row := range rows {
		resp.Result.Records = append(resp.Result.Records, map[string]interface{}{
			"type":  row.typeName,
			"count": float64(row.count),
		})
	}
	return resp
}

// ── parseNodes ──────────────────────────────────────────────────────────────

func TestParseNodes_NilResponse(t *testing.T) {
	cloud, k8s := parseNodes(nil)
	if cloud.Count != 0 || k8s.Count != 0 {
		t.Errorf("nil response must yield zero counts, got cloud=%d k8s=%d", cloud.Count, k8s.Count)
	}
}

func TestParseNodes_ZeroCountRowsIgnored(t *testing.T) {
	resp := nodesResp(
		struct {
			typeName string
			count    int
		}{"AWS_EC2_INSTANCE", 0},
		struct {
			typeName string
			count    int
		}{"K8S_CLUSTER", 0},
	)
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 0 || k8s.Count != 0 {
		t.Errorf("zero-count rows must not contribute, got cloud=%d k8s=%d", cloud.Count, k8s.Count)
	}
}

func TestParseNodes_AWSEntitiesGoToCloud(t *testing.T) {
	resp := nodesResp(struct {
		typeName string
		count    int
	}{"AWS_EC2_INSTANCE", 7})
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 7 {
		t.Errorf("AWS_ entities must count as cloud, got cloud.Count=%d", cloud.Count)
	}
	if k8s.Count != 0 {
		t.Errorf("AWS_ entities must not count as k8s, got k8s.Count=%d", k8s.Count)
	}
	if !strings.Contains(cloud.Details, "ec2 instances") {
		t.Errorf("cloud detail should contain humanized AWS type, got %q", cloud.Details)
	}
}

func TestParseNodes_AzureEntitiesGoToCloud(t *testing.T) {
	resp := nodesResp(struct {
		typeName string
		count    int
	}{"AZURE_MICROSOFT_VIRTUAL_MACHINE", 3})
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 3 {
		t.Errorf("AZURE_ entities must count as cloud, got cloud.Count=%d", cloud.Count)
	}
	if k8s.Count != 0 {
		t.Errorf("AZURE_ entities must not count as k8s, got k8s.Count=%d", k8s.Count)
	}
	// AZURE_MICROSOFT_ prefix stripped → "virtual machine" → humanized
	if !strings.Contains(cloud.Details, "virtual machine") {
		t.Errorf("cloud detail should contain humanized Azure type, got %q", cloud.Details)
	}
}

func TestParseNodes_GCPEntitiesGoToCloud(t *testing.T) {
	resp := nodesResp(struct {
		typeName string
		count    int
	}{"GCP_CLOUD_RUN_SERVICE", 4})
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 4 {
		t.Errorf("GCP_ entities must count as cloud, got cloud.Count=%d", cloud.Count)
	}
	if k8s.Count != 0 {
		t.Errorf("GCP_ entities must not count as k8s, got k8s.Count=%d", k8s.Count)
	}
	if !strings.Contains(cloud.Details, "cloud run service") {
		t.Errorf("cloud detail should contain humanized GCP type, got %q", cloud.Details)
	}
}

func TestParseNodes_K8sAndContainerGoToKubernetes(t *testing.T) {
	resp := nodesResp(
		struct {
			typeName string
			count    int
		}{"K8S_CLUSTER", 2},
		struct {
			typeName string
			count    int
		}{"CONTAINER", 5},
	)
	cloud, k8s := parseNodes(resp)
	if k8s.Count != 7 {
		t.Errorf("K8S_ + CONTAINER must count as k8s, got k8s.Count=%d", k8s.Count)
	}
	if cloud.Count != 0 {
		t.Errorf("K8S_/CONTAINER must not count as cloud, got cloud.Count=%d", cloud.Count)
	}
}

func TestParseNodes_MixedCloudProvidersAggregateCount(t *testing.T) {
	resp := nodesResp(
		struct {
			typeName string
			count    int
		}{"AWS_EC2_INSTANCE", 10},
		struct {
			typeName string
			count    int
		}{"AZURE_MICROSOFT_VIRTUAL_MACHINE", 5},
		struct {
			typeName string
			count    int
		}{"GCP_CLOUD_RUN_SERVICE", 3},
		struct {
			typeName string
			count    int
		}{"K8S_NODE", 8},
	)
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 18 {
		t.Errorf("mixed cloud count must be 10+5+3=18, got %d", cloud.Count)
	}
	if k8s.Count != 8 {
		t.Errorf("k8s count must be 8, got %d", k8s.Count)
	}
	// All three providers' breakdowns must appear in details.
	if !strings.Contains(cloud.Details, "ec2 instances") {
		t.Errorf("AWS breakdown missing from cloud details: %q", cloud.Details)
	}
	if !strings.Contains(cloud.Details, "virtual machine") {
		t.Errorf("Azure breakdown missing from cloud details: %q", cloud.Details)
	}
	if !strings.Contains(cloud.Details, "cloud run service") {
		t.Errorf("GCP breakdown missing from cloud details: %q", cloud.Details)
	}
}

func TestParseNodes_UnknownTypesIgnored(t *testing.T) {
	resp := nodesResp(struct {
		typeName string
		count    int
	}{"SERVICE", 99})
	cloud, k8s := parseNodes(resp)
	if cloud.Count != 0 || k8s.Count != 0 {
		t.Errorf("unknown type must not contribute to any section, got cloud=%d k8s=%d", cloud.Count, k8s.Count)
	}
}

func TestParseNodes_CloudDetailsEmptyWhenNoCloudEntities(t *testing.T) {
	resp := nodesResp(struct {
		typeName string
		count    int
	}{"K8S_CLUSTER", 2})
	cloud, _ := parseNodes(resp)
	if cloud.Details != "" {
		t.Errorf("cloud.Details must be empty when no cloud entities present, got %q", cloud.Details)
	}
}

// helper: build a dqlResponse with the given (resource type, count) rows.
func cloudResp(rows ...struct {
	rt    string
	count int
}) *dqlResponse {
	resp := &dqlResponse{}
	for _, row := range rows {
		rec := map[string]interface{}{}
		if row.rt != "" {
			rec["aws.resource.type"] = row.rt
		}
		rec["count"] = float64(row.count) // JSON numbers decode as float64
		resp.Result.Records = append(resp.Result.Records, rec)
	}
	return resp
}

func TestParseCloudPlatformSignals_EmptyInputs(t *testing.T) {
	if got := parseCloudPlatformSignals(nil, nil); got != "" {
		t.Errorf("nil inputs: got %q, want empty string", got)
	}
	if got := parseCloudPlatformSignals(cloudResp(), cloudResp()); got != "" {
		t.Errorf("empty responses: got %q, want empty string", got)
	}
}

func TestParseCloudPlatformSignals_IgnoresMissingResourceType(t *testing.T) {
	// Row with no aws.resource.type and a row with count=0 must be ignored.
	metrics := cloudResp(
		struct {
			rt    string
			count int
		}{"", 100},
		struct {
			rt    string
			count int
		}{"AWS::Lambda::Function", 0},
	)
	if got := parseCloudPlatformSignals(metrics, nil); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestParseCloudPlatformSignals_StripsAWSPrefixAndCountsTypes(t *testing.T) {
	metrics := cloudResp(
		struct {
			rt    string
			count int
		}{"AWS::Lambda::Function", 10},
		struct {
			rt    string
			count int
		}{"AWS::RDS::DBInstance", 5},
	)
	got := parseCloudPlatformSignals(metrics, nil)
	if !strings.Contains(got, "cloud signals (2 types):") {
		t.Errorf("missing type count header in %q", got)
	}
	if !strings.Contains(got, "Lambda::Function") {
		t.Errorf("missing Lambda label in %q", got)
	}
	if !strings.Contains(got, "RDS::DBInstance") {
		t.Errorf("missing RDS label in %q", got)
	}
	if strings.Contains(got, "AWS::") {
		t.Errorf("AWS:: prefix not stripped: %q", got)
	}
}

func TestParseCloudPlatformSignals_DedupsMetricsAndLogsByType(t *testing.T) {
	metrics := cloudResp(struct {
		rt    string
		count int
	}{"AWS::Lambda::Function", 10})
	logs := cloudResp(struct {
		rt    string
		count int
	}{"AWS::Lambda::Function", 5})

	got := parseCloudPlatformSignals(metrics, logs)
	// Same type from both sources must count once.
	if !strings.Contains(got, "cloud signals (1 type):") {
		t.Errorf("dedup failed, got %q", got)
	}
}

func TestParseCloudPlatformSignals_TopFiveWithMoreSuffix(t *testing.T) {
	metrics := cloudResp(
		struct {
			rt    string
			count int
		}{"AWS::A", 100},
		struct {
			rt    string
			count int
		}{"AWS::B", 90},
		struct {
			rt    string
			count int
		}{"AWS::C", 80},
		struct {
			rt    string
			count int
		}{"AWS::D", 70},
		struct {
			rt    string
			count int
		}{"AWS::E", 60},
		struct {
			rt    string
			count int
		}{"AWS::F", 50},
		struct {
			rt    string
			count int
		}{"AWS::G", 40},
	)
	got := parseCloudPlatformSignals(metrics, nil)
	if !strings.Contains(got, "cloud signals (7 types):") {
		t.Errorf("expected 7-type header, got %q", got)
	}
	if !strings.Contains(got, "+2 more") {
		t.Errorf("expected '+2 more' suffix, got %q", got)
	}
	// F and G must not appear in the truncated label list.
	if strings.Contains(got, " F") || strings.Contains(got, " G") {
		t.Errorf("expected F and G to be truncated, got %q", got)
	}
}

func TestParseCloudPlatformSignals_DeterministicOrderOnTies(t *testing.T) {
	// Two types with the same count must render in stable typeName order
	// across multiple invocations (regression: map iteration was random).
	build := func() string {
		metrics := cloudResp(
			struct {
				rt    string
				count int
			}{"AWS::Zebra", 5},
			struct {
				rt    string
				count int
			}{"AWS::Apple", 5},
			struct {
				rt    string
				count int
			}{"AWS::Mango", 5},
		)
		return parseCloudPlatformSignals(metrics, nil)
	}
	first := build()
	for i := 0; i < 20; i++ {
		if got := build(); got != first {
			t.Fatalf("non-deterministic output:\n  first = %q\n  iter %d = %q", first, i, got)
		}
	}
	// Tie-break should be alphabetical typeName ascending.
	apple := strings.Index(first, "Apple")
	mango := strings.Index(first, "Mango")
	zebra := strings.Index(first, "Zebra")
	if !(apple < mango && mango < zebra) {
		t.Errorf("tie-break order wrong: %q (apple=%d mango=%d zebra=%d)", first, apple, mango, zebra)
	}
}

func TestParseCloudPlatformSignals_SingularPlural(t *testing.T) {
	one := cloudResp(struct {
		rt    string
		count int
	}{"AWS::Lambda::Function", 1})
	got := parseCloudPlatformSignals(one, nil)
	if !strings.Contains(got, "(1 type):") {
		t.Errorf("singular form missing, got %q", got)
	}
	if strings.Contains(got, "(1 types):") {
		t.Errorf("incorrect plural for 1, got %q", got)
	}
}

func TestShortResourceType(t *testing.T) {
	cases := map[string]string{
		"AWS::Lambda::Function": "Lambda::Function",
		"AWS::RDS::DBInstance":  "RDS::DBInstance",
		"Lambda::Function":      "Lambda::Function", // already stripped
		"":                      "",
	}
	for in, want := range cases {
		if got := shortResourceType(in); got != want {
			t.Errorf("shortResourceType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1) != "" {
		t.Errorf("plural(1) = %q, want empty", plural(1))
	}
	if plural(0) != "s" || plural(2) != "s" || plural(99) != "s" {
		t.Errorf("plural(n!=1) should be \"s\"")
	}
}

func TestDqlEscapeString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain account id", "123456789012", "123456789012"},
		{"empty", "", ""},
		{"double quote", `12"34`, `12\"34`},
		{"backslash", `a\b`, `a\\b`},
		{"backslash before quote", `a\"b`, `a\\\"b`},
		{"closing quote injection attempt", `" or "1"=="1`, `\" or \"1\"==\"1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dqlEscapeString(tc.in); got != tc.want {
				t.Errorf("dqlEscapeString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPollAllCloudFilter_EscapesAccountID(t *testing.T) {
	// Sanity check: the formatted filter wraps the escaped value in
	// double quotes — so a quote in the input must not terminate the literal.
	got := fmt.Sprintf(`| filter aws.account.id == "%s" `, dqlEscapeString(`"; drop everything`))
	want := `| filter aws.account.id == "\"; drop everything" `
	if got != want {
		t.Errorf("filter clause = %q, want %q", got, want)
	}
}
