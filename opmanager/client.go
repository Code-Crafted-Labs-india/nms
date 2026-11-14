package opmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	listDevicesPath  = "/api/json/v2/device/listDevices"
	maxResponseBytes = 2 << 20
	maxPageSize      = 100
)

var ErrNotConfigured = errors.New("OpManager integration is not configured")

// Client is a read-only client for the documented OpManager REST API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// Device is the read-only subset of an OpManager device record used by the
// integration. Unknown vendor fields are intentionally ignored.
type Device struct {
	ID             string `json:"id"`
	MOID           int64  `json:"moid"`
	DeviceName     string `json:"deviceName"`
	DisplayName    string `json:"displayName"`
	IPAddress      string `json:"ipaddress"`
	VendorName     string `json:"vendorName"`
	Type           string `json:"type"`
	Category       string `json:"category"`
	Status         string `json:"statusStr"`
	StatusNumber   string `json:"statusNum"`
	InterfaceCount int    `json:"interfaceCount"`
	IsSNMP         bool   `json:"isSNMP"`
	IsSuppressed   bool   `json:"isSuppressed"`
}

// DevicePage is one bounded page from OpManager.
type DevicePage struct {
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	Records int      `json:"records"`
	Rows    []Device `json:"rows"`
}

// NewClient returns nil when both settings are empty. Supplying only one
// setting is a configuration error. The caller owns the API key and must not
// log or expose it.
func NewClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" && apiKey == "" {
		return nil, nil
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("OPMANAGER_BASE_URL and OPMANAGER_API_KEY must both be set")
	}
	if strings.ContainsAny(apiKey, "\r\n") {
		return nil, errors.New("OPMANAGER_API_KEY contains invalid characters")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil ||
		(parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("OPMANAGER_BASE_URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return nil, errors.New("OPMANAGER_BASE_URL must use HTTPS except for loopback development")
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// Never forward the API key to a redirect destination.
				return http.ErrUseLastResponse
			},
		}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    httpClient,
	}, nil
}

// ListDevices retrieves one page of devices. The API key is sent in the
// documented header and is never placed in the request URL.
func (c *Client) ListDevices(ctx context.Context, page, pageSize int) (DevicePage, error) {
	if c == nil {
		return DevicePage{}, ErrNotConfigured
	}
	if page < 1 {
		return DevicePage{}, errors.New("page must be positive")
	}
	if pageSize < 1 || pageSize > maxPageSize {
		return DevicePage{}, fmt.Errorf("page size must be between 1 and %d", maxPageSize)
	}

	endpoint, err := url.JoinPath(c.baseURL, listDevicesPath)
	if err != nil {
		return DevicePage{}, errors.New("invalid OpManager API URL")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return DevicePage{}, errors.New("invalid OpManager API URL")
	}
	query := parsed.Query()
	query.Set("page", strconv.Itoa(page))
	query.Set("rows", strconv.Itoa(pageSize))
	parsed.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return DevicePage{}, errors.New("could not construct OpManager API request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("apikey", c.apiKey)

	response, err := c.http.Do(request)
	if err != nil {
		return DevicePage{}, fmt.Errorf("OpManager API request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return DevicePage{}, fmt.Errorf("OpManager API returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return DevicePage{}, errors.New("could not read OpManager API response")
	}
	if len(body) > maxResponseBytes {
		return DevicePage{}, errors.New("OpManager API response exceeded the size limit")
	}

	var result DevicePage
	if err := json.Unmarshal(body, &result); err != nil {
		return DevicePage{}, errors.New("OpManager API returned invalid JSON")
	}
	if result.Rows == nil {
		result.Rows = []Device{}
	}
	return result, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
