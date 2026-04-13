package installer

import (
	"testing"
)

func TestMapRuntimeToTechtype(t *testing.T) {
	tests := []struct {
		runtime  string
		wantType string
		wantOK   bool
	}{
		{"nodejs18.x", "nodejs", true},
		{"nodejs20.x", "nodejs", true},
		{"python3.12", "python", true},
		{"python3.9", "python", true},
		{"java21", "java", true},
		{"java17", "java", true},
		{"go1.x", "go", true},
		{"dotnet8", "", false},
		{"ruby3.3", "", false},
		{"provided.al2023", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		gotType, gotOK := mapRuntimeToTechtype(tt.runtime)
		if gotType != tt.wantType || gotOK != tt.wantOK {
			t.Errorf("mapRuntimeToTechtype(%q) = (%q, %v), want (%q, %v)",
				tt.runtime, gotType, gotOK, tt.wantType, tt.wantOK)
		}
	}
}

func TestArchToDTArch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"arm64", "arm"},
		{"x86_64", "x86"},
		{"", "x86"},
	}
	for _, tt := range tests {
		got := archToDTArch(tt.input)
		if got != tt.want {
			t.Errorf("archToDTArch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyFunction(t *testing.T) {
	tests := []struct {
		name string
		fn   lambdaFunction
		want string
	}{
		{
			name: "new — supported runtime, no DT layer",
			fn: lambdaFunction{
				Name:    "my-func",
				Runtime: "nodejs18.x",
				Layers:  []string{"arn:aws:lambda:us-east-1:123:layer:SomeOther:1"},
			},
			want: "new",
		},
		{
			name: "update — supported runtime, has DT layer",
			fn: lambdaFunction{
				Name:    "my-func",
				Runtime: "python3.12",
				Layers:  []string{"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_x86:1"},
			},
			want: "update",
		},
		{
			name: "skip — unsupported runtime",
			fn: lambdaFunction{
				Name:    "my-func",
				Runtime: "dotnet8",
				Layers:  nil,
			},
			want: "skip",
		},
		{
			name: "skip — unsupported runtime even with DT layer",
			fn: lambdaFunction{
				Name:    "my-func",
				Runtime: "provided.al2023",
				Layers:  []string{"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_x86:1"},
			},
			want: "skip",
		},
		{
			name: "skip — Dynatrace internal function",
			fn: lambdaFunction{
				Name:    "StackSet-dtaws-12345-DynatraceApiClientFunction-AbCdEf",
				Runtime: "nodejs18.x",
				Layers:  nil,
			},
			want: "skip",
		},
		{
			name: "skip — Dynatrace internal even with DT layer",
			fn: lambdaFunction{
				Name:    "DynatraceApiClientFunction",
				Runtime: "python3.12",
				Layers:  []string{"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_python_x86:1"},
			},
			want: "skip",
		},
		{
			name: "new — no layers at all",
			fn: lambdaFunction{
				Name:    "my-func",
				Runtime: "java21",
				Layers:  nil,
			},
			want: "new",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFunction(tt.fn)
			if got != tt.want {
				t.Errorf("classifyFunction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasDynatraceLayer(t *testing.T) {
	tests := []struct {
		layers []string
		want   bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"arn:aws:lambda:us-east-1:123:layer:SomeOther:1"}, false},
		{[]string{"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_x86:1"}, true},
		{[]string{"arn:aws:lambda:us-east-1:123:layer:Other:1", "arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_python_arm:2"}, true},
	}
	for _, tt := range tests {
		got := hasDynatraceLayer(tt.layers)
		if got != tt.want {
			t.Errorf("hasDynatraceLayer(%v) = %v, want %v", tt.layers, got, tt.want)
		}
	}
}

func TestIsInstrumented(t *testing.T) {
	instrumented := lambdaFunction{
		Layers: []string{"arn:aws:lambda:eu-central-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_arm:1"},
	}
	notInstrumented := lambdaFunction{
		Layers: []string{"arn:aws:lambda:us-east-1:123:layer:SomeLayer:1"},
	}
	empty := lambdaFunction{}

	if !isInstrumented(instrumented) {
		t.Error("isInstrumented should return true for function with DT layer")
	}
	if isInstrumented(notInstrumented) {
		t.Error("isInstrumented should return false for function without DT layer")
	}
	if isInstrumented(empty) {
		t.Error("isInstrumented should return false for function with no layers")
	}
}

func TestMergeDTEnvVars(t *testing.T) {
	conn := &dtConnectionInfo{
		TenantUUID: "abc12345",
		ClusterID:  "997993252",
		BaseURL:    "https://abc12345.live.dynatrace.com",
		Token:      "dt0c01.mytoken",
	}

	t.Run("preserves existing vars", func(t *testing.T) {
		existing := map[string]string{
			"MY_VAR":       "my-value",
			"ANOTHER_VAR":  "another-value",
			"DATABASE_URL": "postgres://localhost:5432/db",
		}

		merged := mergeDTEnvVars(existing, conn, "python3.12")

		// All original vars must be preserved.
		for k, v := range existing {
			if merged[k] != v {
				t.Errorf("existing var %q = %q, want %q", k, merged[k], v)
			}
		}

		// DT vars must be set.
		if merged["AWS_LAMBDA_EXEC_WRAPPER"] != "/opt/dynatrace" {
			t.Errorf("AWS_LAMBDA_EXEC_WRAPPER = %q, want /opt/dynatrace", merged["AWS_LAMBDA_EXEC_WRAPPER"])
		}
		if merged["DT_TENANT"] != "abc12345" {
			t.Errorf("DT_TENANT = %q, want abc12345", merged["DT_TENANT"])
		}
		if merged["DT_CLUSTER"] != "997993252" {
			t.Errorf("DT_CLUSTER = %q, want 997993252", merged["DT_CLUSTER"])
		}
		if merged["DT_CONNECTION_BASE_URL"] != "https://abc12345.live.dynatrace.com" {
			t.Errorf("DT_CONNECTION_BASE_URL = %q", merged["DT_CONNECTION_BASE_URL"])
		}
		if merged["DT_CONNECTION_AUTH_TOKEN"] != "dt0c01.mytoken" {
			t.Errorf("DT_CONNECTION_AUTH_TOKEN = %q", merged["DT_CONNECTION_AUTH_TOKEN"])
		}
		// Non-node runtime must NOT set DT_ENABLE_ESM_LOADERS.
		if _, ok := merged["DT_ENABLE_ESM_LOADERS"]; ok {
			t.Errorf("DT_ENABLE_ESM_LOADERS must not be set for non-node runtime")
		}
	})

	t.Run("nodejs sets DT_ENABLE_ESM_LOADERS", func(t *testing.T) {
		merged := mergeDTEnvVars(map[string]string{}, conn, "nodejs20.x")
		if merged["DT_ENABLE_ESM_LOADERS"] != "true" {
			t.Errorf("DT_ENABLE_ESM_LOADERS = %q, want true", merged["DT_ENABLE_ESM_LOADERS"])
		}
	})

	t.Run("empty existing", func(t *testing.T) {
		merged := mergeDTEnvVars(map[string]string{}, conn, "python3.12")
		if len(merged) != 5 {
			t.Errorf("expected 5 env vars, got %d", len(merged))
		}
		if merged["DT_CLUSTER"] != "997993252" {
			t.Errorf("DT_CLUSTER = %q, want 997993252", merged["DT_CLUSTER"])
		}
	})
}

func TestRemoveDTEnvVars(t *testing.T) {
	t.Run("removes DT vars, preserves others", func(t *testing.T) {
		existing := map[string]string{
			"MY_VAR":                   "keep",
			"DATABASE_URL":             "keep",
			"AWS_LAMBDA_EXEC_WRAPPER":  "/opt/dynatrace",
			"DT_TENANT":                "abc12345",
			"DT_CLUSTER":               "997993252",
			"DT_CONNECTION_BASE_URL":   "https://abc12345.live.dynatrace.com",
			"DT_CONNECTION_AUTH_TOKEN": "dt0c01.mytoken",
		}

		cleaned := removeDTEnvVars(existing)

		if len(cleaned) != 2 {
			t.Errorf("expected 2 vars after removal, got %d", len(cleaned))
		}
		if cleaned["MY_VAR"] != "keep" {
			t.Errorf("MY_VAR should be preserved, got %q", cleaned["MY_VAR"])
		}
		if cleaned["DATABASE_URL"] != "keep" {
			t.Errorf("DATABASE_URL should be preserved, got %q", cleaned["DATABASE_URL"])
		}
		for _, dtKey := range dtEnvVarKeys {
			if _, ok := cleaned[dtKey]; ok {
				t.Errorf("DT key %q should have been removed", dtKey)
			}
		}
	})

	t.Run("empty map after removing only DT vars", func(t *testing.T) {
		existing := map[string]string{
			"AWS_LAMBDA_EXEC_WRAPPER":  "/opt/dynatrace",
			"DT_TENANT":                "abc12345",
			"DT_CONNECTION_BASE_URL":   "https://abc12345.live.dynatrace.com",
			"DT_CONNECTION_AUTH_TOKEN": "dt0c01.mytoken",
		}

		cleaned := removeDTEnvVars(existing)
		if len(cleaned) != 0 {
			t.Errorf("expected 0 vars, got %d", len(cleaned))
		}
	})

	t.Run("no DT vars present — returns all", func(t *testing.T) {
		existing := map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		}

		cleaned := removeDTEnvVars(existing)
		if len(cleaned) != 2 {
			t.Errorf("expected 2 vars, got %d", len(cleaned))
		}
	})
}

func TestUpdateLayers(t *testing.T) {
	newARN := "arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_340_nodejs_x86:1"

	t.Run("appends when no DT layer exists", func(t *testing.T) {
		existing := []string{
			"arn:aws:lambda:us-east-1:123:layer:SomeLayer:1",
			"arn:aws:lambda:us-east-1:456:layer:AnotherLayer:2",
		}
		result := updateLayers(existing, newARN)
		if len(result) != 3 {
			t.Fatalf("expected 3 layers, got %d", len(result))
		}
		if result[2] != newARN {
			t.Errorf("expected new ARN appended, got %q", result[2])
		}
	})

	t.Run("replaces existing DT layer", func(t *testing.T) {
		existing := []string{
			"arn:aws:lambda:us-east-1:123:layer:SomeLayer:1",
			"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_x86:1",
			"arn:aws:lambda:us-east-1:456:layer:AnotherLayer:2",
		}
		result := updateLayers(existing, newARN)
		if len(result) != 3 {
			t.Fatalf("expected 3 layers (replaced), got %d", len(result))
		}
		if result[0] != existing[0] {
			t.Errorf("first layer changed: %q", result[0])
		}
		if result[1] != newARN {
			t.Errorf("DT layer not replaced: got %q, want %q", result[1], newARN)
		}
		if result[2] != existing[2] {
			t.Errorf("third layer changed: %q", result[2])
		}
	})

	t.Run("empty layers — appends", func(t *testing.T) {
		result := updateLayers(nil, newARN)
		if len(result) != 1 || result[0] != newARN {
			t.Errorf("expected [%q], got %v", newARN, result)
		}
	})
}

func TestRemoveDynatraceLayers(t *testing.T) {
	t.Run("removes DT layers, keeps others", func(t *testing.T) {
		layers := []string{
			"arn:aws:lambda:us-east-1:123:layer:SomeLayer:1",
			"arn:aws:lambda:us-east-1:657959507023:layer:Dynatrace_OneAgent_1_338_nodejs_x86:1",
			"arn:aws:lambda:us-east-1:456:layer:AnotherLayer:2",
		}
		result := removeDynatraceLayers(layers)
		if len(result) != 2 {
			t.Fatalf("expected 2 layers, got %d", len(result))
		}
		if result[0] != layers[0] || result[1] != layers[2] {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("no DT layers — returns all", func(t *testing.T) {
		layers := []string{
			"arn:aws:lambda:us-east-1:123:layer:SomeLayer:1",
		}
		result := removeDynatraceLayers(layers)
		if len(result) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(result))
		}
	})

	t.Run("nil layers — returns nil", func(t *testing.T) {
		result := removeDynatraceLayers(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}

func TestIsDynatraceInternal(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"StackSet-dtaws-12345-DynatraceApiClientFunction-AbCdEf", true},
		{"DynatraceApiClientFunction", true},
		{"my-DynatraceApiClientFunction-suffix", true},
		{"my-func", false},
		{"dynatraceapiclientfunction", false}, // case-sensitive
		{"", false},
	}
	for _, tt := range tests {
		fn := lambdaFunction{Name: tt.name}
		got := isDynatraceInternal(fn)
		if got != tt.want {
			t.Errorf("isDynatraceInternal(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSkipReason(t *testing.T) {
	tests := []struct {
		name string
		fn   lambdaFunction
		want string
	}{
		{
			name: "Dynatrace internal",
			fn:   lambdaFunction{Name: "StackSet-DynatraceApiClientFunction-123", Runtime: "nodejs18.x"},
			want: "Dynatrace internal",
		},
		{
			name: "unsupported runtime",
			fn:   lambdaFunction{Name: "my-func", Runtime: "dotnet8"},
			want: "unsupported runtime",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipReason(tt.fn)
			if got != tt.want {
				t.Errorf("skipReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
