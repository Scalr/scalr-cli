package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// setRetryDelay overrides retryBaseDelay for fast tests and returns a cleanup func.
func setRetryDelay(t *testing.T, d time.Duration) func() {
	t.Helper()
	old := retryBaseDelay
	retryBaseDelay = d
	return func() { retryBaseDelay = old }
}

func TestDoWithRetry_Success(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := doWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoWithRetry_RetriesOn5xx(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error": "temporary"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	// Suppress retry log messages
	captureStderr(t, func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := doWithRetry(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected final 200, got %d", resp.StatusCode)
		}
	})

	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoWithRetry_DoesNotRetryOn4xx(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "bad request"}`)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := doWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("4xx should not retry; expected 1 call, got %d", calls)
	}
}

func TestDoWithRetry_RetriesOnNetworkError(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	// Point to a non-routable address that will fail to connect quickly.
	// Using port 1 on localhost is reliable across platforms.
	captureStderr(t, func() {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:1/", nil)
		resp, err := doWithRetry(req)

		if err == nil {
			t.Error("expected an error after all retries")
			if resp != nil {
				resp.Body.Close()
			}
		}
	})
}

// TestDoWithRetry_POSTBodyPreserved verifies the fix for the retry body-reset bug.
// Before the fix, the request body was consumed on the first attempt and subsequent
// retries sent empty bodies.
func TestDoWithRetry_POSTBodyPreserved(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	var receivedBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	expectedBody := `{"data": {"type": "workspaces", "attributes": {"name": "test"}}}`

	captureStderr(t, func() {
		req, _ := http.NewRequest("POST", server.URL, strings.NewReader(expectedBody))
		resp, err := doWithRetry(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
	})

	if len(receivedBodies) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(receivedBodies))
	}
	for i, body := range receivedBodies {
		if body != expectedBody {
			t.Errorf("call %d received body %q, want %q", i, body, expectedBody)
		}
	}
}

func TestDoWithRetry_AllRetriesExhausted(t *testing.T) {
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"errors": [{"status": "500", "title": "persistent failure"}]}`)
	}))
	defer server.Close()

	var resp *http.Response
	var err error
	captureStderr(t, func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err = doWithRetry(req)
	})

	if err != nil {
		t.Fatalf("expected response (not error) after all retries, got err=%v", err)
	}
	if resp == nil {
		t.Fatal("expected response on exhausted retries, got nil")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	// The final response body MUST be readable — earlier bug drained it.
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("failed to read final response body: %v", readErr)
	}
	if !strings.Contains(string(body), "persistent failure") {
		t.Errorf("final response body should be preserved, got %q", string(body))
	}

	// Expected: maxRetries=3 means 1 initial + 3 retries = 4 calls
	if atomic.LoadInt32(&calls) != 4 {
		t.Errorf("expected 4 total calls (1 + maxRetries 3), got %d", calls)
	}
}

func TestDoWithRetry_MixedErrors(t *testing.T) {
	// Simulate: network error, then 500, then 200
	defer setRetryDelay(t, 1*time.Millisecond)()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		// First call fails at the handler level
		if n == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close() // abruptly close connection
			return
		}
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	captureStderr(t, func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := doWithRetry(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("expected final 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}
