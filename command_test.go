package main

import (
	"testing"
)

func stringPtr(value string) *string {
	return &value
}

func TestBuildCreateRunVariablesFromVariableFlags(t *testing.T) {
	flags := map[string]Parameter{
		"variables-key": {
			value: stringPtr("image_tag"),
		},
		"variables-value": {
			value: stringPtr("pr-123"),
		},
		"variables-category": {
			value: stringPtr("terraform"),
		},
		"variables-sensitive": {
			value: stringPtr("true"),
		},
		"variables-hcl": {
			value: stringPtr("false"),
		},
	}

	variables := buildCreateRunVariables(flags)
	if len(variables) != 1 {
		t.Fatalf("expected one variable object, got %d", len(variables))
	}

	variable := variables[0].(map[string]any)
	if variable["key"] != "image_tag" {
		t.Fatalf("expected key to be image_tag, got %#v", variable["key"])
	}

	if variable["value"] != "pr-123" {
		t.Fatalf("expected value to be pr-123, got %#v", variable["value"])
	}

	if variable["category"] != "terraform" {
		t.Fatalf("expected category to be terraform, got %#v", variable["category"])
	}

	if variable["sensitive"] != true {
		t.Fatalf("expected sensitive to be true, got %#v", variable["sensitive"])
	}

	if variable["hcl"] != false {
		t.Fatalf("expected hcl to be false, got %#v", variable["hcl"])
	}
}

func TestBuildCreateRunVariablesFromInputsFlags(t *testing.T) {
	flags := map[string]Parameter{
		"inputs-name": {
			value: stringPtr("image_tag"),
		},
		"inputs-value": {
			value: stringPtr("pr-123"),
		},
		"inputs-sensitive": {
			value: stringPtr("false"),
		},
	}

	variables := buildCreateRunVariables(flags)
	if len(variables) != 1 {
		t.Fatalf("expected one variable object, got %d", len(variables))
	}

	variable := variables[0].(map[string]any)
	if variable["key"] != "image_tag" {
		t.Fatalf("expected key to be image_tag, got %#v", variable["key"])
	}

	if variable["value"] != "pr-123" {
		t.Fatalf("expected value to be pr-123, got %#v", variable["value"])
	}

	if variable["category"] != "terraform" {
		t.Fatalf("expected category to default to terraform, got %#v", variable["category"])
	}

	if variable["sensitive"] != false {
		t.Fatalf("expected sensitive to be false, got %#v", variable["sensitive"])
	}
}

func TestBuildCreateRunVariablesPrefersExplicitVariableFlags(t *testing.T) {
	flags := map[string]Parameter{
		"variables-key": {
			value: stringPtr("explicit"),
		},
		"variables-value": {
			value: stringPtr("value-from-variables"),
		},
		"variables-category": {
			value: stringPtr("env"),
		},
		"inputs-name": {
			value: stringPtr("ignored"),
		},
		"inputs-value": {
			value: stringPtr("ignored"),
		},
	}

	variables := buildCreateRunVariables(flags)
	variable := variables[0].(map[string]any)

	if variable["key"] != "explicit" {
		t.Fatalf("expected explicit key to win, got %#v", variable["key"])
	}

	if variable["value"] != "value-from-variables" {
		t.Fatalf("expected explicit value to win, got %#v", variable["value"])
	}

	if variable["category"] != "env" {
		t.Fatalf("expected explicit category to win, got %#v", variable["category"])
	}
}
