package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func configurationHTTPRequest(method, body string) (*http.Request, *httptest.ResponseRecorder) {
	request := httptest.NewRequest(method, "http://console.local/api/v1/configuration", strings.NewReader(body))
	request.Host = "console.local"
	request.Header.Set("Origin", "http://console.local")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, httptest.NewRecorder()
}

func configurationIdentityRow(hostname, deviceType, sysDescr, sysObject, observedAt string) fakeConfigurationRowResult {
	return fakeConfigurationRowResult{values: []any{hostname, deviceType, sysDescr, sysObject, observedAt}}
}

func TestConfigurationDevicesHandlerDatabaseOutcomes(t *testing.T) {
	queryFailure := errors.New("query failed")
	rowFailure := errors.New("scan failed")
	rowsFailure := errors.New("iteration failed")
	tests := []struct {
		name     string
		database *fakeConfigurationQueries
		wantCode int
		wantBody string
	}{
		{
			name:     "query error",
			database: &fakeConfigurationQueries{queryErr: queryFailure},
			wantCode: http.StatusInternalServerError,
			wantBody: "configuration devices unavailable",
		},
		{
			name:     "scan error",
			database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{values: [][]any{{1}}, scanErr: rowFailure}},
			wantCode: http.StatusInternalServerError,
			wantBody: "configuration devices unavailable",
		},
		{
			name:     "iteration error",
			database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{err: rowsFailure}},
			wantCode: http.StatusInternalServerError,
			wantBody: "configuration devices unavailable",
		},
		{
			name:     "successful empty result",
			database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{}},
			wantCode: http.StatusOK,
			wantBody: `{"devices":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, recorder := configurationHTTPRequest(http.MethodGet, "")
			configurationDevicesHandler(tt.database).ServeHTTP(recorder, request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf("response body %q does not contain %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestConfigurationDevicesHandlerReturnsDetectedProfiles(t *testing.T) {
	rows := &fakeConfigurationRows{values: [][]any{
		{7, "ats-edge", "192.0.2.7", "ATS-3900-24P6X", "ATS-3900-24P6X AOS 3.3.1.1", ".1.3.6.1.4.1.12345", "2026-10-15T12:00:00Z"},
		{8, "unknown-edge", "192.0.2.8", "unknown_discovered", "ExampleOS version 5.4.2", "", "2026-10-15T12:00:00Z"},
	}}
	database := &fakeConfigurationQueries{rows: rows}
	request, recorder := configurationHTTPRequest(http.MethodGet, "")
	configurationDevicesHandler(database).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Devices []configurationDevice `json:"devices"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(response.Devices))
	}
	if response.Devices[0].OSDetection.Profile != "ats3900-aos@3.3.1.1" {
		t.Errorf("ATS device profile = %q", response.Devices[0].OSDetection.Profile)
	}
	if response.Devices[1].OSDetection.Status != "unsupported" || response.Devices[1].OSDetection.Profile != "" {
		t.Errorf("unknown device was not blocked: %#v", response.Devices[1].OSDetection)
	}
	if !rows.closed {
		t.Error("device rows were not closed")
	}
	if !strings.Contains(database.lastQuery, "time >= NOW() - INTERVAL '15 minutes'") || !strings.Contains(database.lastQuery, "ORDER BY time DESC LIMIT 1") {
		t.Errorf("device identity query does not enforce freshness and latest observation: %s", database.lastQuery)
	}
}

func TestReadConfigurationDeviceUsesFreshestMatchingAddressAndMode(t *testing.T) {
	database := &fakeConfigurationQueries{rowResults: []fakeConfigurationRowResult{
		configurationIdentityRow("ats-edge", "ATS-3900-24P6X", "ATS-3900-24P6X AOS 3.3.1.1", ".1.3.6.1.4.1.12345", "2026-10-15T12:00:00Z"),
	}}
	identity, err := readConfigurationDeviceFrom(context.Background(), database, 42)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Hostname != "ats-edge" || identity.DeviceType != "ATS-3900-24P6X" || identity.SysDescr == "" {
		t.Errorf("unexpected device identity: %#v", identity)
	}
	if len(database.rowQueries) != 1 || database.lastRowArgs[0] != 42 {
		t.Fatalf("device read query args = %#v", database.lastRowArgs)
	}
	query := database.rowQueries[0]
	for _, required := range []string{
		`agent_host = host(d.ip_address)`,
		`time >= NOW() - INTERVAL '15 minutes'`,
		`ORDER BY time DESC LIMIT 1`,
		deviceModePredicate("d"),
	} {
		if !strings.Contains(query, required) {
			t.Errorf("device identity query missing %q", required)
		}
	}
}

func TestConfigurationPlansGetDatabaseOutcomes(t *testing.T) {
	queryFailure := errors.New("query failed")
	rowFailure := errors.New("scan failed")
	rowsFailure := errors.New("iteration failed")
	planRow := []any{
		int64(21), 7, "ats-edge", "ats3900-aos@3.3.1.1", "create_vlan",
		json.RawMessage(`{"vlan_id":10}`), []string{"config", "vlan-view 10", "quit"},
		"draft", "operator", "2026-10-15T12:00:00Z",
	}
	tests := []struct {
		name     string
		database *fakeConfigurationQueries
		wantCode int
		wantBody string
	}{
		{name: "query error", database: &fakeConfigurationQueries{queryErr: queryFailure}, wantCode: http.StatusInternalServerError, wantBody: "configuration plans unavailable"},
		{name: "scan error", database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{values: [][]any{{21}}, scanErr: rowFailure}}, wantCode: http.StatusInternalServerError, wantBody: "configuration plans unavailable"},
		{name: "iteration error", database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{err: rowsFailure}}, wantCode: http.StatusInternalServerError, wantBody: "configuration plans unavailable"},
		{name: "successful empty result", database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{}}, wantCode: http.StatusOK, wantBody: `{"plans":[]}`},
		{name: "successful plan result", database: &fakeConfigurationQueries{rows: &fakeConfigurationRows{values: [][]any{planRow}}}, wantCode: http.StatusOK, wantBody: "ats3900-aos@3.3.1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, recorder := configurationHTTPRequest(http.MethodGet, "")
			configurationPlansHandler(tt.database).ServeHTTP(recorder, request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf("response body %q does not contain %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestConfigurationPlansPostDatabaseAndProfileOutcomes(t *testing.T) {
	queryFailure := errors.New("identity query failed")
	insertFailure := errors.New("insert failed")
	identity := configurationIdentityRow("ats-edge", "ATS-3900-24P6X", "ATS-3900-24P6X AOS 3.3.1.1", ".1.3.6.1.4.1.12345", "2026-10-15T12:00:00Z")
	tests := []struct {
		name      string
		body      string
		rows      []fakeConfigurationRowResult
		wantCode  int
		wantError string
	}{
		{name: "device missing", body: `{"device_id":2}`, rows: []fakeConfigurationRowResult{{err: pgx.ErrNoRows}}, wantCode: http.StatusNotFound, wantError: "device not found"},
		{name: "identity query failure", body: `{"device_id":2}`, rows: []fakeConfigurationRowResult{{err: queryFailure}}, wantCode: http.StatusInternalServerError, wantError: "could not identify"},
		{name: "unidentified OS", body: `{"device_id":2}`, rows: []fakeConfigurationRowResult{{values: []any{"edge", "ATS-3900-24P6X", "", "", ""}}}, wantCode: http.StatusUnprocessableEntity, wantError: "No recent SNMP system description"},
		{name: "unsupported firmware", body: `{"device_id":2}`, rows: []fakeConfigurationRowResult{configurationIdentityRow("edge", "ATS-3900-24P6X", "ATS-3900-24P6X AOS 3.3.1.2", "", "2026-10-15T12:00:00Z")}, wantCode: http.StatusUnprocessableEntity, wantError: "no qualified command profile"},
		{name: "profile mismatch", body: `{"device_id":2,"profile":"routing-v3.3.1"}`, rows: []fakeConfigurationRowResult{identity}, wantCode: http.StatusConflict, wantError: "does not match"},
		{name: "invalid VLAN operation parameters", body: `{"device_id":2,"vlan_id":1}`, rows: []fakeConfigurationRowResult{identity}, wantCode: http.StatusUnprocessableEntity, wantError: "VLAN ID must be"},
		{name: "insert failure", body: `{"device_id":2,"operation":"create_vlan","vlan_id":10,"vlan_name":"users"}`, rows: []fakeConfigurationRowResult{identity, {err: insertFailure}}, wantCode: http.StatusInternalServerError, wantError: "could not save"},
		{name: "successful draft derives profile from current identity", body: `{"device_id":2,"operation":"create_vlan","vlan_id":10,"vlan_name":"users"}`, rows: []fakeConfigurationRowResult{identity, {values: []any{int64(32)}}}, wantCode: http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := &fakeConfigurationQueries{rowResults: append([]fakeConfigurationRowResult(nil), tt.rows...)}
			request, recorder := configurationHTTPRequest(http.MethodPost, tt.body)
			request = request.WithContext(context.WithValue(request.Context(), sessionContextKey{}, session{Actor: "pilot-operator"}))
			configurationPlansHandler(database).ServeHTTP(recorder, request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			if tt.wantError != "" && !strings.Contains(recorder.Body.String(), tt.wantError) {
				t.Errorf("response body %q does not contain %q", recorder.Body.String(), tt.wantError)
			}
			if tt.wantCode == http.StatusCreated {
				var response map[string]any
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response["profile"] != "ats3900-aos@3.3.1.1" || response["status"] != "draft" {
					t.Errorf("success response did not use detected profile/draft status: %#v", response)
				}
				if len(database.lastRowArgs) != 6 || database.lastRowArgs[1] != "ats3900-aos@3.3.1.1" || database.lastRowArgs[5] != "pilot-operator" {
					t.Errorf("insert arguments omitted detected profile or actor: %#v", database.lastRowArgs)
				}
				if !reflect.DeepEqual(response["commands"], []any{"config", "vlan-view 10", "name users", "quit"}) {
					t.Errorf("unexpected preview commands: %#v", response["commands"])
				}
			}
		})
	}
}

func TestConfigurationPlanPostRejectsQueryErrorOnInsert(t *testing.T) {
	database := &fakeConfigurationQueries{
		rowResults: []fakeConfigurationRowResult{
			configurationIdentityRow("ats-edge", "ATS-3900-24P6X", "ATS-3900-24P6X AOS 3.3.1.1", "", "2026-10-15T12:00:00Z"),
			{err: errors.New("insert unavailable")},
		},
	}
	request, recorder := configurationHTTPRequest(http.MethodPost, `{"device_id":1,"operation":"create_vlan","vlan_id":10}`)
	configurationPlansHandler(database).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
