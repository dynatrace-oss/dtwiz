package installer

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
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

func TestParseServices(t *testing.T) {
	resp := &dqlResponse{}
	for _, name := range []string{"checkout", "catalog", "payment", "shipping", "frontend", "worker"} {
		resp.Result.Records = append(resp.Result.Records, map[string]interface{}{"name": name})
	}

	sec := parseServices(resp)
	if sec.Count != 6 {
		t.Fatalf("service count = %d, want 6", sec.Count)
	}
	if sec.Link == "" {
		t.Fatal("service link must be set")
	}
	if !strings.Contains(sec.Details, "checkout, catalog, payment, shipping, frontend +1 more") {
		t.Fatalf("service details = %q", sec.Details)
	}
}

func TestParseServices_Empty(t *testing.T) {
	if got := parseServices(nil); got.Count != 0 || got.Details != "" || got.Link == "" {
		t.Fatalf("parseServices(nil) = %#v, want empty section with link", got)
	}
}

func TestParseRelationships(t *testing.T) {
	resp := &dqlResponse{}
	for _, row := range []struct {
		typeName string
		count    interface{}
	}{
		{typeName: "CALLS", count: float64(1200)},
		{typeName: "RUNS_ON", count: json.Number("25")},
		{typeName: "IGNORED", count: 0},
	} {
		resp.Result.Records = append(resp.Result.Records, map[string]interface{}{"type": row.typeName, "count": row.count})
	}

	sec := parseRelationships(resp)
	if sec.Count != 1225 {
		t.Fatalf("relationship count = %d, want 1225", sec.Count)
	}
	if !strings.Contains(sec.Details, "1,200 calls") || !strings.Contains(sec.Details, "25 runs on") {
		t.Fatalf("relationship details = %q", sec.Details)
	}
}

func TestParseExceptions(t *testing.T) {
	sec := parseExceptions(&dqlResponse{Result: struct {
		Records []map[string]interface{} `json:"records"`
	}{Records: []map[string]interface{}{{"count": "42"}}}})
	if sec.Count != 42 {
		t.Fatalf("exception count = %d, want 42", sec.Count)
	}
	if sec.Link == "" {
		t.Fatal("exception link must be set")
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{duration: 3 * time.Second, want: "3s"},
		{duration: 65 * time.Second, want: "1m 5s"},
	}
	for _, tt := range tests {
		if got := formatElapsed(tt.duration); got != tt.want {
			t.Fatalf("formatElapsed(%s) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}

func TestFormatCountAndToInt(t *testing.T) {
	if got := formatCount(1234567); got != "1,234,567" {
		t.Fatalf("formatCount() = %q, want 1,234,567", got)
	}
	for _, tt := range []struct {
		input interface{}
		want  int
	}{
		{input: float64(12), want: 12},
		{input: json.Number("34"), want: 34},
		{input: 56, want: 56},
		{input: "78", want: 78},
		{input: "bad", want: 0},
		{input: nil, want: 0},
	} {
		if got := toInt(tt.input); got != tt.want {
			t.Fatalf("toInt(%#v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTermHyperlink(t *testing.T) {
	if got := termHyperlink("https://example.com", "Example", false); got != "Example (https://example.com)" {
		t.Fatalf("plain hyperlink = %q", got)
	}
	if got := termHyperlink("https://example.com", "Example", true); !strings.Contains(got, "\033]8;;https://example.com") || !strings.Contains(got, "Example") {
		t.Fatalf("tty hyperlink = %q", got)
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

// ── dqlFromLiteral ─────────────────────────────────────────────────────────

func TestDqlFromLiteral_RelativeExpression(t *testing.T) {
	// DQL relative expressions contain parentheses and must not be quoted.
	for _, in := range []string{"now()-1h", "now()-5m", "now()"} {
		if got := dqlFromLiteral(in); got != in {
			t.Errorf("dqlFromLiteral(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestDqlFromLiteral_AbsoluteTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2024-01-15T10:30:00Z", `"2024-01-15T10:30:00Z"`},
		{"2024-06-01T00:00:00+02:00", `"2024-06-01T00:00:00+02:00"`},
	}
	for _, tc := range cases {
		if got := dqlFromLiteral(tc.in); got != tc.want {
			t.Errorf("dqlFromLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── parseLogs ──────────────────────────────────────────────────────────────

// logsResp builds a dqlResponse for the logs query (loglevel + count fields).
func logsResp(rows ...struct {
	level string
	count int
}) *dqlResponse {
	resp := &dqlResponse{}
	for _, row := range rows {
		resp.Result.Records = append(resp.Result.Records, map[string]interface{}{
			"loglevel": row.level,
			"count":    float64(row.count),
		})
	}
	return resp
}

func TestParseLogs_NilResponse(t *testing.T) {
	sec := parseLogs(nil)
	if sec.Count != 0 || sec.Details != "" {
		t.Errorf("nil response must yield empty section, got count=%d details=%q", sec.Count, sec.Details)
	}
}

func TestParseLogs_EmptyRecords(t *testing.T) {
	sec := parseLogs(&dqlResponse{})
	if sec.Count != 0 {
		t.Errorf("empty records must yield Count=0, got %d", sec.Count)
	}
}

func TestParseLogs_SingleLevel(t *testing.T) {
	resp := logsResp(struct {
		level string
		count int
	}{"info", 42})
	sec := parseLogs(resp)
	if sec.Count != 42 {
		t.Errorf("Count = %d, want 42", sec.Count)
	}
	if !strings.Contains(sec.Details, "42 info") {
		t.Errorf("Details = %q, want '42 info'", sec.Details)
	}
}

func TestParseLogs_AllLevelsBreakdown(t *testing.T) {
	resp := logsResp(
		struct {
			level string
			count int
		}{"info", 100},
		struct {
			level string
			count int
		}{"warn", 20},
		struct {
			level string
			count int
		}{"error", 5},
	)
	sec := parseLogs(resp)
	if sec.Count != 125 {
		t.Errorf("total count = %d, want 125", sec.Count)
	}
	for _, want := range []string{"100 info", "20 warn", "5 error"} {
		if !strings.Contains(sec.Details, want) {
			t.Errorf("Details = %q, missing %q", sec.Details, want)
		}
	}
}

func TestParseLogs_UnknownLevelCountedNotShown(t *testing.T) {
	// "debug" and empty-level records count toward the total but must not
	// appear in Details (only info/warn/error are surfaced).
	resp := logsResp(
		struct {
			level string
			count int
		}{"info", 10},
		struct {
			level string
			count int
		}{"debug", 50},
		struct {
			level string
			count int
		}{"", 5},
	)
	sec := parseLogs(resp)
	if sec.Count != 65 {
		t.Errorf("all levels must count toward total, got %d want 65", sec.Count)
	}
	if strings.Contains(sec.Details, "debug") || strings.Contains(sec.Details, "none") {
		t.Errorf("unknown levels must not appear in Details: %q", sec.Details)
	}
}

func TestParseLogs_ZeroCountLevelOmittedFromDetails(t *testing.T) {
	resp := logsResp(
		struct {
			level string
			count int
		}{"info", 0},
		struct {
			level string
			count int
		}{"error", 3},
	)
	sec := parseLogs(resp)
	if sec.Count != 3 {
		t.Errorf("Count = %d, want 3", sec.Count)
	}
	if strings.Contains(sec.Details, "info") {
		t.Errorf("zero-count level must not appear in Details: %q", sec.Details)
	}
	if !strings.Contains(sec.Details, "3 error") {
		t.Errorf("Details = %q, want '3 error'", sec.Details)
	}
}

func TestParseLogs_CaseNormalized(t *testing.T) {
	// DQL may return levels in uppercase; they must normalize for matching.
	resp := logsResp(
		struct {
			level string
			count int
		}{"INFO", 7},
		struct {
			level string
			count int
		}{"WARN", 3},
	)
	sec := parseLogs(resp)
	if sec.Count != 10 {
		t.Errorf("Count = %d, want 10", sec.Count)
	}
	if !strings.Contains(sec.Details, "7 info") {
		t.Errorf("Details = %q, want '7 info'", sec.Details)
	}
	if !strings.Contains(sec.Details, "3 warn") {
		t.Errorf("Details = %q, want '3 warn'", sec.Details)
	}
}

// ── parseRequests ──────────────────────────────────────────────────────────

func TestParseRequests_NilResponse(t *testing.T) {
	sec := parseRequests(nil)
	if sec.Count != 0 || sec.Details != "" {
		t.Errorf("nil response must yield empty section, got count=%d details=%q", sec.Count, sec.Details)
	}
}

func TestParseRequests_EmptyRecords(t *testing.T) {
	sec := parseRequests(&dqlResponse{})
	if sec.Count != 0 {
		t.Errorf("empty records must yield Count=0, got %d", sec.Count)
	}
}

func TestParseRequests_SuccessAndFailed(t *testing.T) {
	resp := &dqlResponse{}
	resp.Result.Records = []map[string]interface{}{
		{"success": float64(95), "failed": float64(5)},
	}
	sec := parseRequests(resp)
	if sec.Count != 100 {
		t.Errorf("Count = %d, want 100", sec.Count)
	}
	if !strings.Contains(sec.Details, "95 successful") {
		t.Errorf("Details = %q, missing '95 successful'", sec.Details)
	}
	if !strings.Contains(sec.Details, "5 failed") {
		t.Errorf("Details = %q, missing '5 failed'", sec.Details)
	}
}

func TestParseRequests_AllSuccessful(t *testing.T) {
	resp := &dqlResponse{}
	resp.Result.Records = []map[string]interface{}{
		{"success": float64(50), "failed": float64(0)},
	}
	sec := parseRequests(resp)
	if sec.Count != 50 {
		t.Errorf("Count = %d, want 50", sec.Count)
	}
	if !strings.Contains(sec.Details, "0 failed") {
		t.Errorf("Details = %q, want '0 failed'", sec.Details)
	}
}

func TestParseRequests_ZeroTotalNoDetails(t *testing.T) {
	resp := &dqlResponse{}
	resp.Result.Records = []map[string]interface{}{
		{"success": float64(0), "failed": float64(0)},
	}
	sec := parseRequests(resp)
	if sec.Count != 0 {
		t.Errorf("Count = %d, want 0", sec.Count)
	}
	if sec.Details != "" {
		t.Errorf("Details should be empty for zero total, got %q", sec.Details)
	}
}

// ── renderSection (Status field) ──────────────────────────────────────────

func TestRenderSection_ShowsCountWhenPositive(t *testing.T) {
	var buf strings.Builder
	noop := color.New()
	linkFn := func(_, label string) string { return label }
	sec := watchSection{Count: 5, Details: "5 info"}
	renderSection(&buf, "Logs", sec, "https://example.com", noop, noop, noop, linkFn)
	out := buf.String()
	if !strings.Contains(out, "(5)") {
		t.Errorf("expected count '(5)' in output, got: %q", out)
	}
	if strings.Contains(out, "waiting") {
		t.Errorf("'waiting...' must not appear when count > 0, got: %q", out)
	}
}

func TestRenderSection_ShowsStatusDuringPhaseTransition(t *testing.T) {
	// When Count == 0 but Status is set (probe just fired or metrics catching up),
	// the status text must be shown instead of "waiting...".
	var buf strings.Builder
	noop := color.New()
	linkFn := func(_, label string) string { return label }
	sec := watchSection{Count: 0, Status: "Logs ingested"}
	renderSection(&buf, "Logs", sec, "https://example.com", noop, noop, noop, linkFn)
	out := buf.String()
	if !strings.Contains(out, "Logs ingested") {
		t.Errorf("expected Status text in output, got: %q", out)
	}
	if strings.Contains(out, "waiting") {
		t.Errorf("'waiting...' must not appear when Status is set, got: %q", out)
	}
}

func TestRenderSection_StatusBranchUsesLink(t *testing.T) {
	// The deep link must be applied to the section title in the Status branch,
	// matching the behaviour of the Count > 0 branch.
	var buf strings.Builder
	noop := color.New()
	var capturedURL string
	linkFn := func(url, label string) string {
		capturedURL = url
		return label
	}
	sec := watchSection{Count: 0, Status: "Logs ingested", Link: "/ui/apps/dynatrace.logs/"}
	renderSection(&buf, "Logs", sec, "https://example.com", noop, noop, noop, linkFn)
	if capturedURL != "https://example.com/ui/apps/dynatrace.logs/" {
		t.Errorf("linkFn called with %q, want full deep-link URL", capturedURL)
	}
}

func TestRenderSection_ShowsWaitingWhenNoCountOrStatus(t *testing.T) {
	var buf strings.Builder
	noop := color.New()
	linkFn := func(_, label string) string { return label }
	sec := watchSection{}
	renderSection(&buf, "Logs", sec, "https://example.com", noop, noop, noop, linkFn)
	out := buf.String()
	if !strings.Contains(out, "waiting") {
		t.Errorf("expected 'waiting...' when no count and no status, got: %q", out)
	}
}
