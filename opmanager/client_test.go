package opmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClientRequiresHTTPSOutsideLoopback(t *testing.T) {
	for _, test := range []struct {
		name    string
		baseURL string
		apiKey  string
	}{
		{name: "missing key", baseURL: "https://nms.example:8060"},
		{name: "missing URL", apiKey: "secret"},
		{name: "remote HTTP", baseURL: "http://nms.example:8060", apiKey: "secret"},
		{name: "URL credentials", baseURL: "https://user:pass@nms.example", apiKey: "secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewClient(test.baseURL, test.apiKey, nil); err == nil {
				t.Fatal("NewClient() unexpectedly accepted invalid settings")
			}
		})
	}

	client, err := NewClient("http://127.0.0.1:8060", "secret", nil)
	if err != nil || client == nil {
		t.Fatalf("NewClient() loopback HTTP = (%v, %v), want a configured client", client, err)
	}
}

func TestListDevicesSendsBoundedReadOnlyRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != listDevicesPath {
			t.Errorf("path = %q, want %q", r.URL.Path, listDevicesPath)
		}
		if got := r.URL.Query().Get("page"); got != "3" {
			t.Errorf("page = %q, want 3", got)
		}
		if got := r.URL.Query().Get("rows"); got != "25" {
			t.Errorf("rows = %q, want 25", got)
		}
		if got := r.Header.Get("apikey"); got != "test-secret" {
			t.Errorf("apikey header = %q, want the configured key", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if strings.Contains(r.URL.RawQuery, "test-secret") {
			t.Error("API key was included in the URL")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":26,"page":3,"records":1,"rows":[{"id":"d-1","deviceName":"core-1","ipaddress":"192.0.2.10","statusStr":"Up"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ListDevices(context.Background(), 3, 25)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 26 || page.Page != 3 || len(page.Rows) != 1 {
		t.Fatalf("page = %+v, want total 26, page 3, one row", page)
	}
	if page.Rows[0].DeviceName != "core-1" || page.Rows[0].IPAddress != "192.0.2.10" {
		t.Fatalf("device = %+v, want decoded device identity", page.Rows[0])
	}
}

func TestListDevicesRejectsInvalidPagingBeforeRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, paging := range [][2]int{{0, 10}, {1, 0}, {1, maxPageSize + 1}} {
		if _, err := client.ListDevices(context.Background(), paging[0], paging[1]); err == nil {
			t.Errorf("ListDevices(%d, %d) unexpectedly succeeded", paging[0], paging[1])
		}
	}
	if called {
		t.Fatal("invalid paging parameters reached the server")
	}
}

func TestListDevicesRejectsBadResponses(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		body        string
		wantErrText string
	}{
		{name: "HTTP error", status: http.StatusUnauthorized, body: `{"message":"bad key"}`, wantErrText: "HTTP 401"},
		{name: "invalid JSON", status: http.StatusOK, body: `{`, wantErrText: "invalid JSON"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, "test-secret", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.ListDevices(context.Background(), 1, 10)
			if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
				t.Fatalf("ListDevices() error = %v, want containing %q", err, test.wantErrText)
			}
		})
	}
}

func TestListDevicesRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListDevices(context.Background(), 1, 10)
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("ListDevices() error = %v, want size-limit error", err)
	}
}

func TestListDevicesDoesNotFollowRedirects(t *testing.T) {
	destinationCalled := false
	destination := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		destinationCalled = true
	}))
	defer destination.Close()

	target, err := url.Parse(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, nil, target.String(), http.StatusFound)
	}))
	defer redirect.Close()

	client, err := NewClient(redirect.URL, "test-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListDevices(context.Background(), 1, 10); err == nil {
		t.Fatal("ListDevices() unexpectedly followed a redirect")
	}
	if destinationCalled {
		t.Fatal("client sent a request to the redirect destination")
	}
}
