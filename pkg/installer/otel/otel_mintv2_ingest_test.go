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

// settingsResponse builds the JSON body for a GET /settings/objects response with one item.
func settingsResponse(objectID, schemaVersion string, value map[string]any) string {
	v, _ := json.Marshal(value)
	return fmt.Sprintf(`{"totalCount":1,"items":[{"objectId":%q,"schemaVersion":%q,"value":%s}]}`,
		objectID, schemaVersion, string(v))
}

func newMintV2TestServer(t *testing.T, getBody string, putStatus int, putBody []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var putCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
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

func newTestMintV2Client(t *testing.T, serverURL string) *sdkMintV2IngestClient {
	t.Helper()
	c, err := newSDKMintV2IngestClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create MINTv2 ingest client: %v", err)
	}
	c.C.HTTP().SetRetryCount(0)
	return c
}

// ─── table-driven tests ────────────────────────────────────────────────────────

func TestMintV2Ingest(t *testing.T) {
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
			name:          "PUT fails — applyMintV2IngestPlan returns error",
			getBody:       settingsResponse("obj-789", "1.6.0", baseValue),
			putStatus:     http.StatusInternalServerError,
			wantEnabled:   false,
			wantPutCalled: true,
			applyWantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, putCalls := newMintV2TestServer(t, tc.getBody, tc.putStatus, nil)
			c := newTestMintV2Client(t, srv.URL)

			objectID, schemaVersion, value, err := c.getMintV2IngestSetting(context.Background())
			if err != nil {
				t.Fatalf("getMintV2IngestSetting() error = %v", err)
			}
			enabled, _ := value["enableMintV2Ingest"].(bool)
			plan := &mintV2IngestPlan{
				enabled:       enabled,
				objectID:      objectID,
				schemaVersion: schemaVersion,
				value:         value,
			}

			if plan.enabled != tc.wantEnabled {
				t.Errorf("plan.enabled = %v, want %v", plan.enabled, tc.wantEnabled)
			}

			err = applyMintV2IngestPlan(context.Background(), c, plan)
			if tc.applyWantErr && err == nil {
				t.Error("applyMintV2IngestPlan() expected error, got nil")
			}
			if !tc.applyWantErr && err != nil {
				t.Errorf("applyMintV2IngestPlan() unexpected error: %v", err)
			}

			if gotPutCalled := len(*putCalls) > 0; gotPutCalled != tc.wantPutCalled {
				t.Errorf("PUT called = %v, want %v", gotPutCalled, tc.wantPutCalled)
			}
		})
	}
}

// TestApplyMintV2IngestPlan_NilPlan verifies nil plan is a no-op.
func TestApplyMintV2IngestPlan_NilPlan(t *testing.T) {
	srv, putCalls := newMintV2TestServer(t, `{"totalCount":0,"items":[]}`, http.StatusOK, nil)
	c := newTestMintV2Client(t, srv.URL)
	if err := applyMintV2IngestPlan(context.Background(), c, nil); err != nil {
		t.Fatalf("apply with nil plan: %v", err)
	}
	if len(*putCalls) > 0 {
		t.Error("PUT should not be called for nil plan")
	}
}

// TestApplyMintV2IngestPlan_PreservesOtherFields verifies the merge PUT sends all
// original fields verbatim alongside enableMintV2Ingest=true.
func TestApplyMintV2IngestPlan_PreservesOtherFields(t *testing.T) {
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

	c := newTestMintV2Client(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	objectID, schemaVersion, value, err := c.getMintV2IngestSetting(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	plan := &mintV2IngestPlan{objectID: objectID, schemaVersion: schemaVersion, value: value}
	if err := applyMintV2IngestPlan(context.Background(), c, plan); err != nil {
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

// TestGetMintV2IngestSetting_EmptyList verifies an error is returned when GET returns no objects.
func TestGetMintV2IngestSetting_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalCount":0,"items":[]}`))
	}))
	defer srv.Close()

	c := newTestMintV2Client(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	_, _, _, err := c.getMintV2IngestSetting(context.Background())
	if err == nil {
		t.Fatal("expected error for empty items list, got nil")
	}
	if !strings.Contains(err.Error(), "no objects returned") {
		t.Errorf("error %q should mention 'no objects returned'", err)
	}
}

// TestGetMintV2IngestSetting_MalformedJSON verifies that a malformed GET response returns an error.
func TestGetMintV2IngestSetting_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	c := newTestMintV2Client(t, srv.URL)
	c.C.HTTP().SetRetryCount(0)

	_, _, _, err := c.getMintV2IngestSetting(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// readAll reads the request body; helper for tests.
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
