package main

import (
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func TestSanitizeCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello", "hello"},
		{"starts with equals", "=SUM(A1)", "'=SUM(A1)"},
		{"starts with plus", "+1234", "'+1234"},
		{"starts with minus", "-1234", "'-1234"},
		{"starts with at", "@formula", "'@formula"},
		{"empty string", "", ""},
		{"contains equals mid", "a=b", "a=b"},
		{"starts with quote", "'safe", "'safe"},
		{"unicode", "héllo", "héllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeCSV(tt.in); got != tt.want {
				t.Errorf("sanitizeCSV(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{"found first", []string{"a", "b", "c"}, "a", true},
		{"found middle", []string{"a", "b", "c"}, "b", true},
		{"found last", []string{"a", "b", "c"}, "c", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"empty string found", []string{""}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsString(tt.slice, tt.val); got != tt.want {
				t.Errorf("containsString(%v, %q) = %v, want %v", tt.slice, tt.val, got, tt.want)
			}
		})
	}
}

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty defaults to json", "", "json"},
		{"json unchanged", "json", "json"},
		{"table lowercase", "table", "table"},
		{"table uppercase normalized", "TABLE", "table"},
		{"table mixed case", "Table", "table"},
		{"csv normalized", "CSV", "csv"},
		{"whitespace trimmed", "  table  ", "table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveFormat(tt.in); got != tt.want {
				t.Errorf("resolveFormat(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatScalar(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int-like float", 42.0, "42"},
		{"negative int-like float", -7.0, "-7"},
		{"non-int float", 3.14, "3.14"},
		{"empty array", []interface{}{}, "[]"},
		{"array of ints", []interface{}{1.0, 2.0, 3.0}, "1,2,3"},
		{"array of strings", []interface{}{"a", "b"}, "a,b"},
		{"map with id", map[string]interface{}{"id": "ws-123", "type": "workspaces"}, "ws-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gabs.New()
			c.Set(tt.data)
			got := formatScalar(c)
			if got != tt.want {
				t.Errorf("formatScalar(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestFormatScalar_NilContainer(t *testing.T) {
	if got := formatScalar(nil); got != "" {
		t.Errorf("formatScalar(nil) = %q, want empty", got)
	}
}

func TestFormatScalar_UnwrapsGabsContainer(t *testing.T) {
	// parseData stores id/type as *gabs.Container wrappers via SetP.
	// formatScalar must unwrap these to show the actual value, not '"env-1"'.
	outer := gabs.New()
	inner := gabs.New()
	inner.Set("env-1")
	outer.Set(inner)

	got := formatScalar(outer)
	if got != "env-1" {
		t.Errorf("expected unwrapped 'env-1', got %q", got)
	}
}

func TestExtractValue(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "name": "prod", "meta": {"flag": true}, "empty": null}`)

	tests := []struct {
		field string
		want  string
	}{
		{"id", "ws-1"},
		{"name", "prod"},
		{"empty", ""},     // null field
		{"missing", ""},   // absent field
		{"meta.flag", "true"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if got := extractValue(item, tt.field); got != tt.want {
				t.Errorf("extractValue(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestFilterSingleObject(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "name": "prod", "status": "applied", "extra": "ignored"}`)

	result := filterSingleObject(item, []string{"id", "name"})
	gotKeys := make(map[string]bool)
	for k := range result.ChildrenMap() {
		gotKeys[k] = true
	}

	if !gotKeys["id"] || !gotKeys["name"] {
		t.Errorf("missing expected keys: got %v", gotKeys)
	}
	if gotKeys["status"] || gotKeys["extra"] {
		t.Errorf("unexpected keys present: got %v", gotKeys)
	}
}

func TestFilterSingleObject_TrimsSpaces(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)
	result := filterSingleObject(item, []string{" id ", " name "})

	if !result.Exists("id") || !result.Exists("name") {
		t.Errorf("whitespace-trimmed fields not found: %v", result.String())
	}
}

func TestFilterSingleObject_MissingFields(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1"}`)
	result := filterSingleObject(item, []string{"id", "name", "nonexistent"})

	if !result.Exists("id") {
		t.Error("id should be present")
	}
	if result.Exists("name") || result.Exists("nonexistent") {
		t.Error("missing fields should not be added as nulls")
	}
}

func TestFilterFields_EmptyFieldsReturnsOriginal(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)
	result := filterFields(data, "", false)
	if result != data {
		t.Error("empty fields should return the original pointer")
	}
}

func TestFilterFields_Array(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "ws-1", "name": "prod", "status": "a"}, {"id": "ws-2", "name": "dev", "status": "b"}]`)
	result := filterFields(data, "id,name", true)

	children := result.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 items, got %d", len(children))
	}
	for i, c := range children {
		if !c.Exists("id") || !c.Exists("name") {
			t.Errorf("item %d missing fields: %s", i, c.String())
		}
		if c.Exists("status") {
			t.Errorf("item %d has unexpected status field", i)
		}
	}
}

func TestAutoDetectColumns_PrefersKnownFields(t *testing.T) {
	item := parseJSONForTest(t, `{"unknown": "x", "id": "ws-1", "name": "prod", "other": "y"}`)
	cols := autoDetectColumns(item)

	// id and name should come before other fields
	idIdx, nameIdx := -1, -1
	for i, c := range cols {
		if c == "id" {
			idIdx = i
		}
		if c == "name" {
			nameIdx = i
		}
	}
	if idIdx == -1 || nameIdx == -1 {
		t.Fatalf("expected id and name in columns, got %v", cols)
	}
	if idIdx > nameIdx {
		t.Errorf("expected id before name, got %v", cols)
	}
}

func TestAutoDetectColumns_SkipsNestedObjects(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "relationships": {"account": {"id": "acc-1"}}, "name": "prod"}`)
	cols := autoDetectColumns(item)

	for _, c := range cols {
		if c == "relationships" {
			t.Error("nested object field should not be in columns")
		}
	}
}

func TestAutoDetectColumns_FallbackWhenAllNested(t *testing.T) {
	item := parseJSONForTest(t, `{"obj1": {"a": 1}, "obj2": {"b": 2}}`)
	cols := autoDetectColumns(item)

	if len(cols) == 0 {
		t.Error("fallback should include at least some columns")
	}
}

func TestAutoDetectColumns_EmptyObject(t *testing.T) {
	item := parseJSONForTest(t, `{}`)
	cols := autoDetectColumns(item)

	if len(cols) != 0 {
		t.Errorf("empty object should return empty cols, got %v", cols)
	}
}

func TestResolveColumns_ExplicitColumnsTakePriority(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "name": "prod", "type": "workspaces"}`)
	cols := resolveColumns(item, "name,id", "workspaces")

	if len(cols) != 2 || cols[0] != "name" || cols[1] != "id" {
		t.Errorf("expected [name id], got %v", cols)
	}
}

func TestResolveColumns_UsesDefaultsForType(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "ws-1", "name": "prod", "status": "applied", "type": "workspaces", "terraform-version": "1.7.0", "auto-apply": true, "execution-mode": "remote"}`)
	cols := resolveColumns(item, "", "")

	// Should detect type from item and use defaultColumns["workspaces"]
	if len(cols) == 0 {
		t.Fatal("expected non-empty cols")
	}
	foundID := false
	for _, c := range cols {
		if c == "id" {
			foundID = true
		}
	}
	if !foundID {
		t.Errorf("expected id in cols, got %v", cols)
	}
}

func TestResolveColumns_TypeFromGabsContainer(t *testing.T) {
	// Simulate what parseData does: SetP wraps type in a container
	item := gabs.New()
	item.Set("prod", "name")
	inner := gabs.New()
	inner.Set("environments")
	item.Set(inner, "type")
	item.Set("env-1", "id")
	item.Set("Active", "status")

	cols := resolveColumns(item, "", "")

	// Should resolve type to "environments" and find defaults
	if len(cols) == 0 {
		t.Fatal("should have found columns")
	}
}

func TestResolveColumns_FallsBackToAutoDetect(t *testing.T) {
	item := parseJSONForTest(t, `{"id": "x-1", "custom-field": "y", "type": "unknown-resource"}`)
	cols := resolveColumns(item, "", "")

	if len(cols) == 0 {
		t.Error("expected auto-detect fallback")
	}
}

// ====================
// Writer-capturing tests
// ====================

func TestFormatKeyValue_SingleObject(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod", "locked": false}`)
	out := captureStdout(t, func() {
		formatKeyValue(data)
	})

	if !strings.Contains(out, "ID:") {
		t.Errorf("expected 'ID:' label, got:\n%s", out)
	}
	if !strings.Contains(out, "ws-1") {
		t.Errorf("expected 'ws-1' value, got:\n%s", out)
	}
	if !strings.Contains(out, "NAME:") {
		t.Errorf("expected 'NAME:' label, got:\n%s", out)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("expected 'prod' value, got:\n%s", out)
	}
}

func TestFormatKeyValue_EmptyContainer(t *testing.T) {
	data := gabs.New() // empty map
	out := captureStdout(t, func() {
		formatKeyValue(data)
	})

	// Empty container should not produce 'No data.' — the callAPI check handles that.
	// Instead, it falls back to raw container output (empty in this case).
	if strings.Contains(out, "No data.") {
		t.Errorf("'No data.' should never appear; got:\n%s", out)
	}
}

func TestFormatTable_List(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "ws-1", "name": "prod"}, {"id": "ws-2", "name": "dev"}]`)
	out := captureStdout(t, func() {
		formatTable(data, "id,name", "")
	})

	if !strings.Contains(out, "ID") || !strings.Contains(out, "NAME") {
		t.Errorf("expected headers, got:\n%s", out)
	}
	if !strings.Contains(out, "ws-1") || !strings.Contains(out, "ws-2") {
		t.Errorf("expected both rows, got:\n%s", out)
	}
	if !strings.Contains(out, "prod") || !strings.Contains(out, "dev") {
		t.Errorf("expected names, got:\n%s", out)
	}
}

func TestFormatTable_EmptyList(t *testing.T) {
	data := gabs.New()
	data.Array()

	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			formatTable(data, "", "")
		})
	})

	if !strings.Contains(stderr, "No results found") {
		t.Errorf("expected 'No results found' on stderr, got:\n%s", stderr)
	}
}

func TestFormatCSV_Array(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "ws-1", "name": "prod"}, {"id": "ws-2", "name": "dev"}]`)
	out := captureStdout(t, func() {
		formatCSV(data, "id,name", "", true)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d:\n%s", len(lines), out)
	}
	if lines[0] != "id,name" {
		t.Errorf("expected 'id,name' header, got %q", lines[0])
	}
}

func TestFormatCSV_SanitizesFormulas(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "=SUM(A1)", "name": "prod"}]`)
	out := captureStdout(t, func() {
		formatCSV(data, "id,name", "", true)
	})

	// Should prefix =SUM(A1) with a single quote
	if !strings.Contains(out, "'=SUM(A1)") {
		t.Errorf("expected formula-injected value to be sanitized, got:\n%s", out)
	}
}

func TestFormatCSV_SingleObject(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)
	out := captureStdout(t, func() {
		formatCSV(data, "", "", false)
	})

	if !strings.Contains(out, "field,value") {
		t.Errorf("expected 'field,value' header for single object, got:\n%s", out)
	}
}
