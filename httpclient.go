package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	maxRetries         = 3
	retryBaseDelay     = 1 * time.Second
)

// scalrHTTPClient returns an http.Client with a sensible timeout.
// The timeout prevents scripts from hanging indefinitely on unresponsive servers.
var scalrHTTPClient = &http.Client{
	Timeout: defaultHTTPTimeout,
}

// doWithRetry executes an HTTP request with automatic retry for transient failures.
// Retries on: 5xx status codes, network errors, and timeouts.
// Does NOT retry on: 4xx (client errors), 3xx (redirects), or successful responses.
func doWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1)) // exponential: 1s, 2s, 4s
			fmt.Fprintf(os.Stderr, "Retrying in %s (attempt %d/%d)...\n", delay, attempt+1, maxRetries+1)
			time.Sleep(delay)
		}

		resp, err := scalrHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			// Network error or timeout — retryable
			continue
		}

		// 5xx = server error, retryable
		if resp.StatusCode >= 500 {
			lastResp = resp
			lastErr = nil
			continue
		}

		// Any other status (2xx, 3xx, 4xx) — return immediately, no retry
		return resp, nil
	}

	// All retries exhausted
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}
