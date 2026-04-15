package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

// captureStdout runs fn and returns what it wrote to os.Stdout.
// Tests using this cannot run in parallel because they swap a global.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	return buf.String()
}

// captureStderr runs fn and returns what it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	return buf.String()
}

// parseJSONForTest builds a gabs container from a JSON string.
// Fails the test if parsing fails.
func parseJSONForTest(t *testing.T, s string) *gabs.Container {
	t.Helper()
	c, err := gabs.ParseJSON([]byte(s))
	if err != nil {
		t.Fatalf("parseJSONForTest: %v\ninput: %s", err, s)
	}
	return c
}

// gabsFromMap builds a gabs container from a Go map literal.
func gabsFromMap(t *testing.T, data map[string]interface{}) *gabs.Container {
	t.Helper()
	c := gabs.New()
	if _, err := c.Set(data); err != nil {
		t.Fatalf("gabsFromMap: %v", err)
	}
	return c
}
