package installer

import (
	"strings"
	"testing"
)

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
