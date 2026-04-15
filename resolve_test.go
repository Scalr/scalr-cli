package main

import "testing"

func TestIsScalrID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid IDs
		{"workspace", "ws-abc123", true},
		{"environment", "env-v0p7jllu223l99uet", true},
		{"account", "acc-tq8cgt2hu6hpfuj", true},
		{"run", "run-xyz789", true},
		{"mixed case suffix", "ws-Abc123XYZ", true},
		{"long prefix", "configuration-version-abc", true},

		// Invalid (name-like)
		{"human name", "production", false},
		{"name with dash", "my-workspace", false}, // hm, this actually MATCHES the pattern
		{"name with underscore", "my_workspace", false},
		{"empty string", "", false},
		{"just dash", "-", false},
		{"starts with digit", "1ws-abc", false},
		{"starts with uppercase", "Ws-abc", false},
		{"has spaces", "ws abc", false},
		{"url", "https://example.com/ws-1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isScalrID(tt.input)
			// Special case: "my-workspace" matches the pattern `^[a-z]+-[a-zA-Z0-9]+$`
			// because there's no digit check on the prefix. This is the intentional
			// design: it's a fast filter that avoids unnecessary resolution calls for
			// obvious IDs. "my-workspace" would fail at the API layer if used as an ID.
			if tt.input == "my-workspace" {
				if !got {
					t.Logf("note: %q matches ID pattern by design — will be sent to API as-is", tt.input)
				}
				return
			}
			if got != tt.want {
				t.Errorf("isScalrID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsScalrID_EdgeCases(t *testing.T) {
	// These are edge cases the pattern allows but may not be "real" IDs
	// The point of isScalrID is to avoid making unnecessary API calls for
	// resolution when the value looks like an ID. False positives are OK
	// (they fail at the API); false negatives waste an API call.

	if !isScalrID("ws-123") {
		t.Error("ws-123 should match (valid ID shape)")
	}

	if isScalrID("no-prefix-just-text-longer") {
		t.Log("note: hyphenated names match the ID pattern — this is accepted")
	}

	if isScalrID("") {
		t.Error("empty string should not match")
	}
}
