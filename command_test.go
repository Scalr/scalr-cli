package main

import (
	"strings"
	"testing"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestRenameFlag(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"page[size]", "page-size"},
		{"filter[name]", "filter-name"},
		{"simple", "simple"},
		{"[]", "-"},
		{"", ""},
		{"nested[a][b]", "nested-a-b"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := renameFlag(tt.in); got != tt.want {
				t.Errorf("renameFlag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestShortenName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"data-attributes-name", "name"},
		{"data-attributes-cost-estimation-enabled", "cost-estimation-enabled"},
		{"data-relationships-account-data-id", "account-id"},
		{"data-relationships-environment-data-id", "environment-id"},
		{"data-id", "id"},
		{"already-short", "already-short"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := shortenName(tt.in); got != tt.want {
				t.Errorf("shortenName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsValidExternalHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		// Valid hosts
		{"scalr.io", "scalr.io", true},
		{"subdomain", "example.scalr.io", true},
		{"deep subdomain", "a.b.c.d.scalr.io", true},

		// Invalid: no dot (single-label)
		{"localhost", "localhost", false},
		{"bare hostname", "myhost", false},
		{"empty", "", false},

		// Invalid: IP addresses
		{"ipv4 localhost", "127.0.0.1", false},
		{"ipv4 private", "10.0.0.1", false},
		{"ipv4 public", "8.8.8.8", false},
		{"ipv6", "::1", false},

		// Invalid: localhost suffixes
		{"scalr.localhost", "scalr.localhost", false},
		{"test.local", "test.local", false},
		{"uppercase localhost", "DEV.LOCALHOST", false},
		{"mixed case local", "Host.Local", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidExternalHost(tt.host); got != tt.want {
				t.Errorf("isValidExternalHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestParseData_SingleItem(t *testing.T) {
	raw := `{
		"data": {
			"id": "env-1",
			"type": "environments",
			"attributes": {
				"name": "production",
				"status": "Active"
			}
		}
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)

	// parseData always returns an array — single items are wrapped
	children := result.Children()
	if len(children) != 1 {
		t.Fatalf("expected 1 item, got %d", len(children))
	}

	item := children[0]
	if !item.Exists("id") || !item.Exists("name") || !item.Exists("status") {
		t.Errorf("missing expected fields: %s", item.String())
	}
}

func TestParseData_List(t *testing.T) {
	raw := `{
		"data": [
			{"id": "ws-1", "type": "workspaces", "attributes": {"name": "prod"}},
			{"id": "ws-2", "type": "workspaces", "attributes": {"name": "dev"}}
		]
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)

	children := result.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 items, got %d", len(children))
	}

	// Each item should have ChildrenMap returning fields (not empty — that was the bug)
	for i, item := range children {
		if len(item.ChildrenMap()) == 0 {
			t.Errorf("item %d: ChildrenMap is empty — container double-wrapping bug", i)
		}
	}
}

func TestParseData_Relationships(t *testing.T) {
	raw := `{
		"data": {
			"id": "ws-1",
			"type": "workspaces",
			"attributes": {"name": "prod"},
			"relationships": {
				"environment": {
					"data": {"id": "env-1", "type": "environments"}
				}
			}
		}
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)
	children := result.Children()

	if len(children) != 1 {
		t.Fatalf("expected 1 item, got %d", len(children))
	}

	item := children[0]
	if !item.Exists("environment") {
		t.Errorf("expected 'environment' relationship field, got %s", item.String())
	}
}

func TestParseData_RelationshipsWithIncluded(t *testing.T) {
	raw := `{
		"data": {
			"id": "ws-1",
			"type": "workspaces",
			"attributes": {"name": "prod"},
			"relationships": {
				"environment": {
					"data": {"id": "env-1", "type": "environments"}
				}
			}
		},
		"included": [
			{
				"id": "env-1",
				"type": "environments",
				"attributes": {"name": "production-env"}
			}
		]
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)
	item := result.Children()[0]

	// With included data, the relationship should have the full attributes (e.g., name)
	envStr := item.Path("environment").String()
	if !strings.Contains(envStr, "production-env") {
		t.Errorf("expected relationship attributes resolved from included, got %s", envStr)
	}
}

func TestParseData_NullRelationships(t *testing.T) {
	// Relationship with data=null (e.g., optional relationship not set)
	raw := `{
		"data": {
			"id": "ws-1",
			"type": "workspaces",
			"attributes": {"name": "prod"},
			"relationships": {
				"optional-rel": {"data": null}
			}
		}
	}`
	response := parseJSONForTest(t, raw)

	// Should not panic
	result := parseData(response)
	if result == nil {
		t.Fatal("parseData returned nil")
	}
}

func TestParseData_ArrayRelationships(t *testing.T) {
	raw := `{
		"data": {
			"id": "ws-1",
			"type": "workspaces",
			"attributes": {"name": "prod"},
			"relationships": {
				"tags": {
					"data": [
						{"id": "tag-1", "type": "tags"},
						{"id": "tag-2", "type": "tags"}
					]
				}
			}
		}
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)
	item := result.Children()[0]

	// Array relationship should appear as an array
	tags := item.Path("tags").Children()
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestParseData_EmptyIncluded(t *testing.T) {
	raw := `{
		"data": {"id": "ws-1", "type": "workspaces", "attributes": {"name": "prod"}},
		"included": []
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)

	if len(result.Children()) != 1 {
		t.Errorf("expected 1 item even with empty included array")
	}
}

func TestParseData_MalformedIncluded(t *testing.T) {
	// Included item missing type/id — must not panic
	raw := `{
		"data": {"id": "ws-1", "type": "workspaces", "attributes": {"name": "prod"}},
		"included": [
			{"attributes": {"foo": "bar"}}
		]
	}`
	response := parseJSONForTest(t, raw)

	// Should not panic thanks to nil guards
	result := parseData(response)
	if len(result.Children()) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Children()))
	}
}

func TestParseData_LinksSection(t *testing.T) {
	raw := `{
		"data": {
			"id": "cv-1",
			"type": "configuration-versions",
			"attributes": {"status": "pending"},
			"links": {"upload-url": "https://upload.example.com/xyz"}
		}
	}`
	response := parseJSONForTest(t, raw)
	result := parseData(response)
	item := result.Children()[0]

	// links section should be preserved
	if !item.Exists("links") {
		t.Errorf("expected 'links' section preserved, got %s", item.String())
	}
	// The links content should contain the upload-url
	linksStr := item.Path("links").String()
	if linksStr == "" || !contains(linksStr, "upload-url") || !contains(linksStr, "https://upload.example.com/xyz") {
		t.Errorf("upload-url not preserved, got %s", linksStr)
	}
}
