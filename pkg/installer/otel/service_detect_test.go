package otel

import (
	"reflect"
	"testing"
)

func TestCollectorTenantsFromConfig(t *testing.T) {
	cfg := []byte(`
exporters:
  otlp_http:
    endpoint: https://abc12345.live.dynatrace.com/api/v2/otlp
  otlp_http/dynatrace:
    endpoint: https://abc12345.live.dynatrace.com/api/v2/otlp
  debug: {}
service: {}
`)
	got := collectorTenantsFromConfig(cfg)
	want := []string{"abc12345"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tenants = %v, want %v", got, want)
	}
}

func TestCollectorTenantsFromConfig_MultipleTenants(t *testing.T) {
	cfg := []byte(`
exporters:
  a:
    endpoint: https://aaa11111.live.dynatrace.com/api/v2/otlp
  b:
    endpoint: https://bbb22222.dev.dynatracelabs.com/api/v2/otlp
`)
	got := collectorTenantsFromConfig(cfg)
	want := []string{"aaa11111", "bbb22222"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tenants = %v, want %v", got, want)
	}
}

func TestCollectorTenantsFromConfig_NoExporters(t *testing.T) {
	if got := collectorTenantsFromConfig([]byte("service: {}\n")); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestCollectorTenantsFromConfig_IgnoresNonDynatraceExporters(t *testing.T) {
	cfg := []byte(`
exporters:
  otlp/dynatrace:
    endpoint: https://abc12345.live.dynatrace.com/api/v2/otlp
  otlp/jaeger:
    endpoint: http://jaeger-collector:4317
  otlp/tempo:
    endpoint: http://tempo:4317
`)
	got := collectorTenantsFromConfig(cfg)
	want := []string{"abc12345"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tenants = %v, want %v", got, want)
	}
}

func TestIsDynatraceEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{"https://abc12345.live.dynatrace.com/api/v2/otlp", true},
		{"https://abc12345.dev.dynatracelabs.com/api/v2/otlp", true},
		{"https://managed.example.com/e/abc12345/api/v2/otlp", true},
		{"http://jaeger-collector:4317", false},
		{"http://localhost:4317", false},
		{"https://grafana-tempo.internal/v1/traces", false},
		{"not-a-url", false},
	}
	for _, tt := range tests {
		if got := isDynatraceEndpoint(tt.endpoint); got != tt.want {
			t.Errorf("isDynatraceEndpoint(%q) = %v, want %v", tt.endpoint, got, tt.want)
		}
	}
}

func TestOtlpEndpointFromEnv(t *testing.T) {
	tests := []struct {
		name   string
		cmdEnv string
		want   string
	}{
		{
			name:   "base endpoint",
			cmdEnv: "/usr/bin/python app.py FOO=bar OTEL_EXPORTER_OTLP_ENDPOINT=https://x.dynatrace.com/api/v2/otlp BAZ=qux",
			want:   "https://x.dynatrace.com/api/v2/otlp",
		},
		{
			name:   "traces endpoint when base absent",
			cmdEnv: "node index.js OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://y.dynatrace.com/otlp",
			want:   "https://y.dynatrace.com/otlp",
		},
		{
			name:   "base preferred over signal-specific",
			cmdEnv: "app OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://traces OTEL_EXPORTER_OTLP_ENDPOINT=https://base",
			want:   "https://base",
		},
		{
			name:   "dt_environment ignored",
			cmdEnv: "dtwiz setup DT_ENVIRONMENT=https://z.dynatrace.com",
			want:   "",
		},
		{
			name:   "not instrumented",
			cmdEnv: "/usr/bin/python app.py FOO=bar",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := otlpEndpointFromEnv(tt.cmdEnv); got != tt.want {
				t.Fatalf("otlpEndpointFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEndpointMatchesCollector(t *testing.T) {
	tenants := map[string]bool{"abc12345": true}
	ports := map[string]bool{"4317": true, "4318": true}

	tests := []struct {
		name     string
		endpoint string
		want     bool
	}{
		{"same tenant direct export", "https://abc12345.live.dynatrace.com/api/v2/otlp", true},
		{"same tenant apps host", "https://abc12345.dev.apps.dynatracelabs.com", true},
		{"different tenant", "https://zzz99999.live.dynatrace.com/api/v2/otlp", false},
		{"local collector http", "http://localhost:4318", true},
		{"local collector grpc", "localhost:4317", true},
		{"local but wrong port", "http://localhost:9999", false},
		{"loopback ip", "http://127.0.0.1:4318/v1/traces", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := endpointMatchesCollector(tt.endpoint, tenants, ports); got != tt.want {
				t.Fatalf("endpointMatchesCollector(%q) = %v, want %v", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestHostPort(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantPort string
	}{
		{"http://localhost:4318", "localhost", "4318"},
		{"https://x.dynatrace.com/api/v2/otlp", "x.dynatrace.com", ""},
		{"localhost:4317", "localhost", "4317"},
		{"127.0.0.1:4318/v1/traces", "127.0.0.1", "4318"},
	}
	for _, tt := range tests {
		h, p := hostPort(tt.in)
		if h != tt.wantHost || p != tt.wantPort {
			t.Fatalf("hostPort(%q) = (%q,%q), want (%q,%q)", tt.in, h, p, tt.wantHost, tt.wantPort)
		}
	}
}

func TestStripEnvSuffix(t *testing.T) {
	in := "/usr/bin/python delivery/app.py DT_ENVIRONMENT=https://x OTEL_EXPORTER_OTLP_HEADERS=Authorization=secret"
	want := "/usr/bin/python delivery/app.py"
	if got := stripEnvSuffix(in); got != want {
		t.Fatalf("stripEnvSuffix() = %q, want %q", got, want)
	}
	// No env: unchanged.
	if got := stripEnvSuffix("node index.js --port 3000"); got != "node index.js --port 3000" {
		t.Fatalf("stripEnvSuffix changed a command with no env: %q", got)
	}
}

func TestServiceDisplayName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/Library/Frameworks/Python.app/Contents/MacOS/Python delivery/app.py", "Python delivery/app.py"},
		{"/usr/bin/node /srv/frontend/server.js", "node frontend/server.js"},
		{"/usr/bin/python3 /opt/svc/worker.py", "python3 worker.py"},
		{"/usr/local/bin/myapp --flag", "myapp"},
	}
	for _, tt := range tests {
		if got := serviceDisplayName(tt.in); got != tt.want {
			t.Fatalf("serviceDisplayName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestReconcileExportEnv_RetargetsToDTEnvironment(t *testing.T) {
	// DT_ENVIRONMENT points at a new tenant; stale OTLP vars must be repointed.
	env := []string{
		"PATH=/usr/bin",
		"DT_ENVIRONMENT=https://newtenant.dev.apps.dynatracelabs.com",
		"DT_PLATFORM_TOKEN=dt0s16.NEWTOKEN",
		"OTEL_EXPORTER_OTLP_ENDPOINT=https://oldtenant.dev.dynatracelabs.com/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Api-Token%20dt0s16.OLDTOKEN",
	}
	out := reconcileExportEnv(env)
	if reflect.DeepEqual(out, env) {
		t.Fatal("expected env to be updated")
	}
	wantEndpoint := "https://newtenant.dev.dynatracelabs.com/api/v2/otlp"
	if got := envGet(out, "OTEL_EXPORTER_OTLP_ENDPOINT"); got != wantEndpoint {
		t.Fatalf("endpoint = %q, want %q", got, wantEndpoint)
	}
	if got := envGet(out, "OTEL_EXPORTER_OTLP_HEADERS"); got != "Authorization=Api-Token%20dt0s16.NEWTOKEN" {
		t.Fatalf("header = %q", got)
	}
	// Unrelated vars preserved.
	if envGet(out, "PATH") != "/usr/bin" {
		t.Fatal("PATH not preserved")
	}
}

func TestReconcileExportEnv_NoopWhenConsistent(t *testing.T) {
	env := []string{
		"DT_ENVIRONMENT=https://t.dev.apps.dynatracelabs.com",
		"DT_PLATFORM_TOKEN=dt0s16.TOK",
		"OTEL_EXPORTER_OTLP_ENDPOINT=https://t.dev.dynatracelabs.com/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Api-Token%20dt0s16.TOK",
	}
	if out := reconcileExportEnv(env); !reflect.DeepEqual(out, env) {
		t.Fatal("expected no change when already consistent")
	}
}

func TestReconcileExportEnv_NoopWhenNoDTEnvironment(t *testing.T) {
	env := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=https://x/api/v2/otlp"}
	if out := reconcileExportEnv(env); !reflect.DeepEqual(out, env) {
		t.Fatal("expected no change without DT_ENVIRONMENT")
	}
}

func TestReconcileExportEnv_NoopForCollectorRoutedApp(t *testing.T) {
	// App routes through local collector; DT_ENVIRONMENT must not override it.
	env := []string{
		"DT_ENVIRONMENT=https://newtenant.dev.apps.dynatracelabs.com",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318",
	}
	if out := reconcileExportEnv(env); !reflect.DeepEqual(out, env) {
		t.Fatal("expected no change for loopback endpoint")
	}
}

func TestReconcileExportEnv_DropsSignalEndpoints(t *testing.T) {
	env := []string{
		"DT_ENVIRONMENT=https://newtenant.dev.apps.dynatracelabs.com",
		"OTEL_EXPORTER_OTLP_ENDPOINT=https://old.dev.dynatracelabs.com/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://old.dev.dynatracelabs.com/v1/traces",
	}
	out := reconcileExportEnv(env)
	if reflect.DeepEqual(out, env) {
		t.Fatal("expected env to be updated")
	}
	if envGet(out, "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" {
		t.Fatal("stale signal endpoint should be dropped")
	}
}

func TestRebuildAuthHeader(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		token    string
		want     string
	}{
		{"swap api-token", "Authorization=Api-Token%20OLD", "NEW", "Authorization=Api-Token%20NEW"},
		{"preserve bearer", "Authorization=Bearer%20OLD", "NEW", "Authorization=Bearer%20NEW"},
		{"add when absent", "", "NEW", "Authorization=Api-Token%20NEW"},
		{"preserve other headers", "X-Foo=bar,Authorization=Api-Token%20OLD", "NEW", "X-Foo=bar,Authorization=Api-Token%20NEW"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rebuildAuthHeader(tt.existing, tt.token); got != tt.want {
				t.Fatalf("rebuildAuthHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnvSetAndRemove(t *testing.T) {
	env := []string{"A=1", "B=2"}
	env = envSet(env, "B", "9")
	env = envSet(env, "C", "3")
	if envGet(env, "B") != "9" || envGet(env, "C") != "3" || envGet(env, "A") != "1" {
		t.Fatalf("envSet wrong: %v", env)
	}
	env = envRemove(env, "A", "C")
	if envGet(env, "A") != "" || envGet(env, "C") != "" || envGet(env, "B") != "9" {
		t.Fatalf("envRemove wrong: %v", env)
	}
}

func TestRetargetEnvToCollector_SetsEndpointAndDropsSignalOverrides(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"OTEL_EXPORTER_OTLP_ENDPOINT=https://rrx28105.dev.dynatracelabs.com/api/v2/otlp",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://rrx28105.dev.dynatracelabs.com/v1/traces",
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20tok",
	}
	out, changed := retargetEnvToCollector(env, "http://localhost:4320")
	if !changed {
		t.Fatal("expected changed=true")
	}
	if got := envGet(out, "OTEL_EXPORTER_OTLP_ENDPOINT"); got != "http://localhost:4320" {
		t.Fatalf("endpoint = %q, want http://localhost:4320", got)
	}
	if got := envGet(out, "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); got != "" {
		t.Fatalf("signal endpoint should be removed, got %q", got)
	}
	// Other vars preserved.
	if envGet(out, "PATH") != "/usr/bin" {
		t.Fatal("PATH not preserved")
	}
	if envGet(out, "OTEL_EXPORTER_OTLP_HEADERS") != "Authorization=Bearer%20tok" {
		t.Fatal("OTLP_HEADERS should be preserved")
	}
}

func TestRetargetEnvToCollector_NoopWhenAlreadyLoopback(t *testing.T) {
	env := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4320"}
	out, changed := retargetEnvToCollector(env, "http://localhost:4320")
	if changed {
		t.Fatal("expected no change when endpoint is already loopback")
	}
	if &out[0] != &env[0] { // same slice returned
		_ = out // just confirm no panic
	}
}

func TestRetargetEnvToCollector_NoopFor127Loopback(t *testing.T) {
	env := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4320"}
	_, changed := retargetEnvToCollector(env, "http://localhost:4320")
	if changed {
		t.Fatal("expected no change for 127.0.0.1 loopback")
	}
}

func TestIsEnvKey(t *testing.T) {
	yes := []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "PATH", "DT_ENVIRONMENT", "A1_B2"}
	no := []string{"", "1FOO", "lowercase", "delivery/app.py", "app.py"}
	for _, s := range yes {
		if !isEnvKey(s) {
			t.Errorf("isEnvKey(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isEnvKey(s) {
			t.Errorf("isEnvKey(%q) = true, want false", s)
		}
	}
}
