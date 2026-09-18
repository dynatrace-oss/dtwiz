package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func settingsResponse(objectID, schemaVersion string, value map[string]any) string {
	v, _ := json.Marshal(value)
	return fmt.Sprintf(`{"totalCount":1,"items":[{"objectId":%q,"schemaVersion":%q,"value":%s}]}`,
		objectID, schemaVersion, string(v))
}

func newOTLPMetricDimensionsTestServer(t *testing.T, getBody string, putStatus int, putBody []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var putCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/settings/schemas/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaId":"builtin:opentelemetry-metrics"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/settings/objects"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getBody))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/settings/objects/"):
			putCalls = append(putCalls, r.URL.Path)
			w.WriteHeader(putStatus)
			if putBody != nil {
				_, _ = w.Write(putBody)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &putCalls
}

func newTestOTLPMetricDimensionsClient(t *testing.T, serverURL string) *sdkOTLPMetricDimensionsClient {
	t.Helper()
	c, err := newSDKOTLPMetricDimensionsClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create OTLP metric dimensions client: %v", err)
	}
	c.C.HTTP().SetRetryCount(0)
	return c
}

func TestOTLPMetricDimensionsApply(t *testing.T) {
	baseValue := map[string]any{
		"enableMintV2Ingest":                     false,
		"additionalAttributesToDimensionEnabled": true,
		"meterNameToDimensionEnabled":            true,
		"additionalAttributes":                   []any{"attr1", "attr2"},
		"toDropAttributes":                       []any{"drop1"},
	}
	enabledValue := map[string]any{
		"enableMintV2Ingest":                     true,
		"additionalAttributesToDimensionEnabled": true,
		"meterNameToDimensionEnabled":            true,
		"additionalAttributes":                   []any{"attr1", "attr2"},
		"toDropAttributes":                       []any{"drop1"},
	}

	cases := []struct {
		name          string
		getBody       string
		putStatus     int
		wantEnabled   bool
		wantPutCalled bool
		applyWantErr  bool
	}{
		{
			name:          "already enabled — no PUT",
			getBody:       settingsResponse("obj-123", "1.6.2", enabledValue),
			putStatus:     http.StatusOK,
			wantEnabled:   true,
			wantPutCalled: false,
		},
		{
			name:          "disabled — PUT with enableMintV2Ingest true, other fields preserved",
			getBody:       settingsResponse("obj-456", "1.6.1", baseValue),
			putStatus:     http.StatusOK,
			wantEnabled:   false,
			wantPutCalled: true,
		},
		{
			name:          "PUT fails — applyOTLPMetricDimensionsPlan returns error",
			getBody:       settingsResponse("obj-789", "1.6.0", baseValue),
			putStatus:     http.StatusInternalServerError,
			wantEnabled:   false,
			wantPutCalled: true,
			applyWantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, putCalls := newOTLPMetricDimensionsTestServer(t, tc.getBody, tc.putStatus, nil)
			c := newTestOTLPMetricDimensionsClient(t, srv.URL)

			objectID, schemaVersion, value, err := c.getOTLPMetricDimensionsSetting(context.Background())
			if err != nil {
				t.Fatalf("getOTLPMetricDimensionsSetting() error = %v", err)
			}
			enabled, _ := value["enableMintV2Ingest"].(bool)
			plan := &otlpMetricDimensionsPlan{
				enabled:       enabled,
				objectID:      objectID,
				schemaVersion: schemaVersion,
				value:         value,
			}

			if plan.enabled != tc.wantEnabled {
				t.Errorf("plan.enabled = %v, want %v", plan.enabled, tc.wantEnabled)
			}

			err = applyOTLPMetricDimensionsPlan(context.Background(), c, plan)
			if tc.applyWantErr && err == nil {
				t.Error("applyOTLPMetricDimensionsPlan() expected error, got nil")
			}
			if !tc.applyWantErr && err != nil {
				t.Errorf("applyOTLPMetricDimensionsPlan() unexpected error: %v", err)
			}

			if gotPutCalled := len(*putCalls) > 0; gotPutCalled != tc.wantPutCalled {
				t.Errorf("PUT called = %v, want %v", gotPutCalled, tc.wantPutCalled)
			}
		})
	}
}

func TestApplyOTLPMetricDimensionsPlan_NilPlan(t *testing.T) {
	srv, putCalls := newOTLPMetricDimensionsTestServer(t, `{"totalCount":0,"items":[]}`, http.StatusOK, nil)
	c := newTestOTLPMetricDimensionsClient(t, srv.URL)
	if err := applyOTLPMetricDimensionsPlan(context.Background(), c, nil); err != nil {
		t.Fatalf("apply with nil plan: %v", err)
	}
	if len(*putCalls) > 0 {
		t.Error("PUT should not be called for nil plan")
	}
}

func TestApplyOTLPMetricDimensionsPlan_PreservesOtherFields(t *testing.T) {
	original := map[string]any{
		"enableMintV2Ingest":                     false,
		"additionalAttributesToDimensionEnabled": true,
		"meterNameToDimensionEnabled":            false,
		"additionalAttributes":                   []any{"x", "y", "z"},
		"toDropAttributes":                       []any{"a"},
	}
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(settingsResponse("obj-pf", "1.6.2", original)))
		case http.MethodPut:
			capturedBody, _ = readAll(r)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestOTLPMetricDimensionsClient(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	objectID, schemaVersion, value, err := c.getOTLPMetricDimensionsSetting(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	plan := &otlpMetricDimensionsPlan{objectID: objectID, schemaVersion: schemaVersion, value: value}
	if err := applyOTLPMetricDimensionsPlan(context.Background(), c, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var body struct {
		Value map[string]any `json:"value"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("parse PUT body: %v", err)
	}
	if v, _ := body.Value["enableMintV2Ingest"].(bool); !v {
		t.Error("PUT body: enableMintV2Ingest should be true")
	}
	if v, _ := body.Value["additionalAttributesToDimensionEnabled"].(bool); !v {
		t.Error("PUT body: additionalAttributesToDimensionEnabled should be preserved as true")
	}
	if v, _ := body.Value["meterNameToDimensionEnabled"].(bool); v {
		t.Error("PUT body: meterNameToDimensionEnabled should be preserved as false")
	}
	attrs, _ := body.Value["additionalAttributes"].([]any)
	if len(attrs) != 3 {
		t.Errorf("PUT body: additionalAttributes has %d entries, want 3", len(attrs))
	}
}

func TestGetOTLPMetricDimensionsSetting_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalCount":0,"items":[]}`))
	}))
	defer srv.Close()

	c := newTestOTLPMetricDimensionsClient(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	_, _, _, err := c.getOTLPMetricDimensionsSetting(context.Background())
	if err == nil {
		t.Fatal("expected error for empty items list, got nil")
	}
	if !strings.Contains(err.Error(), "no objects returned") {
		t.Errorf("error %q should mention 'no objects returned'", err)
	}
}

func TestGetOTLPMetricDimensionsSetting_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	c := newTestOTLPMetricDimensionsClient(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	_, _, _, err := c.getOTLPMetricDimensionsSetting(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestBuildOTLPMetricDimensionsPlan_Phase3 verifies that 404 from the schema registry
// returns nil, nil, nil — the schema was removed from this tenant type (phase 3).
func TestBuildOTLPMetricDimensionsPlan_Phase3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, plan, err := buildOTLPMetricDimensionsPlan(srv.URL, "dt0s16.test")
	if err != nil {
		t.Fatalf("phase 3 should return nil error, got: %v", err)
	}
	if plan != nil {
		t.Errorf("phase 3 should return nil plan, got: %+v", plan)
	}
	if c != nil {
		t.Error("phase 3 should return nil client")
	}
}

// TestBuildOTLPMetricDimensionsPlan_ObjectsNotFound verifies that 404 from the objects
// endpoint is a real error when the schema is confirmed present — not a phase-3 skip.
func TestBuildOTLPMetricDimensionsPlan_ObjectsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/settings/schemas/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaId":"builtin:opentelemetry-metrics"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	_, _, err := buildOTLPMetricDimensionsPlan(srv.URL, "dt0s16.test")
	if err == nil {
		t.Fatal("expected error when schema exists but objects endpoint returns 404, got nil")
	}
}

func readAll(r *http.Request) ([]byte, error) {
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}
