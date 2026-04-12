package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultHTTPTimeout = 5 * time.Minute // generous timeout — some Scalr operations are slow
	maxRetries         = 3
	retryBaseDelay     = 1 * time.Second
)

// scalrHTTPClient is an http.Client with a sensible timeout.
// The timeout prevents scripts from hanging indefinitely on unresponsive servers.
var scalrHTTPClient = &http.Client{
	Timeout: defaultHTTPTimeout,
}

// doWithRetry executes an HTTP request with automatic retry for transient failures.
// Retries on: 5xx status codes, network errors, and timeouts.
// Does NOT retry on: 4xx (client errors), 3xx (redirects), or successful responses.
// Properly resets the request body between retries for POST/PATCH/DELETE.
func doWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1)) // exponential: 1s, 2s, 4s
			fmt.Fprintf(os.Stderr, "Retrying in %s (attempt %d/%d)...\n", delay, attempt+1, maxRetries+1)
			time.Sleep(delay)

			// Reset the request body for the retry — the previous attempt consumed it.
			// GetBody is set by http.NewRequest when the body is a *strings.Reader,
			// *bytes.Reader, or *bytes.Buffer, which covers all our callAPI usage.
			if req.GetBody != nil {
				newBody, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("cannot reset request body for retry: %w", err)
				}
				req.Body = newBody
			}
		}

		resp, err := scalrHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			// Network error or timeout — retryable
			continue
		}

		// 5xx = server error, retryable
		if resp.StatusCode >= 500 {
			// Close the body from the previous failed response to avoid resource leaks,
			// but only if this is NOT the final attempt — the caller needs the body
			// from the final response to display the error.
			if attempt < maxRetries {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			lastResp = resp
			lastErr = nil
			continue
		}

		// Any other status (2xx, 3xx, 4xx) — return immediately, no retry
		return resp, nil
	}

	// All retries exhausted — return last 5xx response so caller can read the error body
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}
