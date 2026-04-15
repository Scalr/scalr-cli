package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// setHost temporarily sets global Scalr config for tests against a mock server.
func setHost(t *testing.T, serverURL string) func() {
	t.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("bad server URL: %v", err)
	}

	oldHost := ScalrHostname
	oldBase := BasePath
	oldToken := ScalrToken
	oldAccount := ScalrAccount

	ScalrHostname = u.Host
	BasePath = ""
	ScalrToken = "test-token"
	ScalrAccount = "acc-test"

	return func() {
		ScalrHostname = oldHost
		BasePath = oldBase
		ScalrToken = oldToken
		ScalrAccount = oldAccount
	}
}

// mockServer wraps httptest.NewServer but promotes to HTTPS via a small trick:
// since doWithRetry's URL is built with hardcoded "https://" prefix, we need a
// TLS server. For tests we use NewTLSServer and point the client's transport
// at it.
//
// Simpler alternative: use the existing scalrHTTPClient but replace it for tests.
func withHTTPSClient(t *testing.T, server *httptest.Server) func() {
	t.Helper()
	old := scalrHTTPClient
	scalrHTTPClient = server.Client()
	return func() { scalrHTTPClient = old }
}

func TestResolveNameToID_AlreadyAnID(t *testing.T) {
	// When the value matches the ID pattern, no API call should happen.
	// We use an invalid host to verify: if resolution attempted it would fail.
	ScalrHostname = "this-should-not-be-called-xxx-invalid"

	got := resolveNameToID("workspace", "ws-abc123")
	if got != "ws-abc123" {
		t.Errorf("expected ID passthrough, got %q", got)
	}
}

func TestResolveNameToID_UnknownFlagName(t *testing.T) {
	// Flag name is not in resolvableResources — should return as-is
	got := resolveNameToID("some-random-flag", "not-an-id-value")
	if got != "not-an-id-value" {
		t.Errorf("unknown flag should pass through, got %q", got)
	}
}

func TestResolveNameToID_EmptyValue(t *testing.T) {
	got := resolveNameToID("workspace", "")
	if got != "" {
		t.Errorf("empty value should pass through, got %q", got)
	}
}

func TestResolveNameToID_SingleMatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify filter[name] was URL-encoded
		name := r.URL.Query().Get("filter[name]")
		if name != "production" {
			t.Errorf("expected filter[name]=production, got %q", name)
		}

		w.Header().Set("Content-Type", "application/vnd.api+json")
		fmt.Fprintf(w, `{"data": [{"id": "env-resolved", "type": "environments", "attributes": {"name": "production"}}]}`)
	}))
	defer server.Close()

	defer setHost(t, server.URL)()
	defer withHTTPSClient(t, server)()

	// Capture stderr because resolveNameToID prints a message on success
	captureStderr(t, func() {
		got := resolveNameToID("environment", "production")
		if got != "env-resolved" {
			t.Errorf("expected 'env-resolved', got %q", got)
		}
	})
}

func TestResolveNameToID_URLEncodesSpecialChars(t *testing.T) {
	// Verifies the URL encoding fix: values with & or = must not break the query
	var rawURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawURL = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/vnd.api+json")
		fmt.Fprintf(w, `{"data": [{"id": "env-1", "type": "environments", "attributes": {"name": "a & b"}}]}`)
	}))
	defer server.Close()

	defer setHost(t, server.URL)()
	defer withHTTPSClient(t, server)()

	captureStderr(t, func() {
		resolveNameToID("environment", "a & b")
	})

	// The filter parameter should be URL-encoded, not literal "a & b"
	if !strings.Contains(rawURL, "filter%5Bname%5D=a+%26+b") && !strings.Contains(rawURL, "filter%5Bname%5D=a%20%26%20b") {
		t.Errorf("value should be URL-encoded, got raw query: %s", rawURL)
	}
}
