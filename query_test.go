package main

import (
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func TestIsSimpleValue(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want bool
	}{
		{"string", "hello", true},
		{"bool", true, true},
		{"number", 42.0, true},
		{"map", map[string]interface{}{"a": 1}, false},
		{"array", []interface{}{1, 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gabs.New()
			c.Set(tt.data)
			if got := isSimpleValue(c); got != tt.want {
				t.Errorf("isSimpleValue(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestApplyQuery_EmptyQuery(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)
	result, isSimple := applyQuery(data, "", false)

	if isSimple {
		t.Error("empty query should not be simple")
	}
	if result != data {
		t.Error("empty query should return original data")
	}
}

func TestApplyQuery_SingleFieldFromObject(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)
	result, isSimple := applyQuery(data, ".name", false)

	if !isSimple {
		t.Error("extracting a string field should be simple")
	}
	if s, ok := result.Data().(string); !ok || s != "prod" {
		t.Errorf("expected 'prod', got %v", result.Data())
	}
}

func TestApplyQuery_NestedFieldFromObject(t *testing.T) {
	data := parseJSONForTest(t, `{"account": {"id": "acc-1", "name": "My Acc"}}`)
	result, isSimple := applyQuery(data, ".account.id", false)

	if !isSimple {
		t.Error("nested field extraction should be simple")
	}
	if s, _ := result.Data().(string); s != "acc-1" {
		t.Errorf("expected 'acc-1', got %v", result.Data())
	}
}

func TestApplyQuery_MissingField(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1"}`)
	result, isSimple := applyQuery(data, ".nonexistent", false)

	if !isSimple {
		t.Error("missing field should still be simple (just empty)")
	}
	// Result should be empty container
	if result.Data() != nil && result.String() != "{}" {
		t.Errorf("expected empty result for missing field, got %v", result.String())
	}
}

func TestApplyQuery_ArrayIteration(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "ws-1"}, {"id": "ws-2"}, {"id": "ws-3"}]`)
	result, isSimple := applyQuery(data, ".[].id", true)

	if !isSimple {
		t.Error("array of strings should be simple")
	}
	children := result.Children()
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
	got := []string{}
	for _, c := range children {
		if s, ok := c.Data().(string); ok {
			got = append(got, s)
		}
	}
	want := []string{"ws-1", "ws-2", "ws-3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestApplyQuery_ArrayImplicitWithoutBrackets(t *testing.T) {
	// When data is an array and query is `.field` (not `.[].field`), it should still work.
	data := parseJSONForTest(t, `[{"name": "a"}, {"name": "b"}]`)
	result, _ := applyQuery(data, ".name", true)

	children := result.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestApplyQuery_JustBrackets(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "a"}, {"id": "b"}]`)
	result, _ := applyQuery(data, ".[]", true)

	// `.[]` alone returns the array itself
	if len(result.Children()) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Children()))
	}
}

func TestApplyQuery_ArrayWithNestedPath(t *testing.T) {
	data := parseJSONForTest(t, `[{"meta": {"key": "val-1"}}, {"meta": {"key": "val-2"}}]`)
	result, isSimple := applyQuery(data, ".[].meta.key", true)

	if !isSimple {
		t.Error("nested simple values should be simple")
	}
	children := result.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestApplyQuery_ArrayMissingFieldInSome(t *testing.T) {
	data := parseJSONForTest(t, `[{"id": "a"}, {"other": "b"}, {"id": "c"}]`)
	result, _ := applyQuery(data, ".[].id", true)

	// Should only include items that have the id field
	children := result.Children()
	if len(children) != 2 {
		t.Errorf("expected 2 matching items, got %d", len(children))
	}
}

func TestApplyQuery_ObjectField(t *testing.T) {
	data := parseJSONForTest(t, `{"nested": {"a": 1, "b": 2}}`)
	result, isSimple := applyQuery(data, ".nested", false)

	if isSimple {
		t.Error("an object result should not be simple")
	}
	if !result.Exists("a") || !result.Exists("b") {
		t.Errorf("expected nested object, got %v", result.String())
	}
}

func TestExtractFromArray_Empty(t *testing.T) {
	data := gabs.New()
	data.Array()
	result, isSimple := extractFromArray(data, "id")

	if isSimple {
		t.Error("empty array should not be simple")
	}
	if len(result.Children()) != 0 {
		t.Errorf("expected empty result, got %d items", len(result.Children()))
	}
}

func TestFormatQueryResult_SimpleArray(t *testing.T) {
	result := gabs.New()
	result.Array()
	result.ArrayAppend("a")
	result.ArrayAppend("b")

	out := captureStdout(t, func() {
		formatQueryResult(result, true)
	})

	// Should print one per line
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), out)
	}
	if lines[0] != "a" || lines[1] != "b" {
		t.Errorf("got %v, want [a b]", lines)
	}
}

func TestFormatQueryResult_SingleSimple(t *testing.T) {
	c := gabs.New()
	c.Set("hello")

	out := captureStdout(t, func() {
		formatQueryResult(c, true)
	})

	if strings.TrimSpace(out) != "hello" {
		t.Errorf("got %q, want 'hello'", out)
	}
}

func TestFormatQueryResult_Complex(t *testing.T) {
	data := parseJSONForTest(t, `{"id": "ws-1", "name": "prod"}`)

	out := captureStdout(t, func() {
		formatQueryResult(data, false)
	})

	// Complex result should be JSON-formatted
	if !strings.Contains(out, `"id"`) || !strings.Contains(out, `"ws-1"`) {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}
