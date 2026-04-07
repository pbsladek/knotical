package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSLToJSONSchema(t *testing.T) {
	schema, err := DSLToJSONSchema("name, age:int, active:bool")
	if err != nil {
		t.Fatalf("DSLToJSONSchema failed: %v", err)
	}

	properties := schema["properties"].(map[string]any)
	if properties["name"].(map[string]any)["type"] != "string" {
		t.Fatalf("expected name to default to string")
	}
	if properties["age"].(map[string]any)["type"] != "integer" {
		t.Fatalf("expected age to be integer")
	}
	if properties["active"].(map[string]any)["type"] != "boolean" {
		t.Fatalf("expected active to be boolean")
	}
}

func TestValidate(t *testing.T) {
	schema, err := DSLToJSONSchema("name, age:int")
	if err != nil {
		t.Fatalf("DSLToJSONSchema failed: %v", err)
	}

	if err := Validate(schema, map[string]any{"name": "alice", "age": float64(42)}); err != nil {
		t.Fatalf("Validate rejected valid payload: %v", err)
	}

	if err := Validate(schema, map[string]any{"name": "alice"}); err == nil {
		t.Fatalf("Validate accepted missing field")
	}
}

func TestValidateUsesRealJSONSchemaFeatures(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"meta": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind": map[string]any{
						"type": "string",
						"enum": []any{"incident", "deploy"},
					},
				},
				"required": []any{"kind"},
			},
		},
		"required": []any{"name", "tags", "meta"},
	}

	valid := map[string]any{
		"name": "alice",
		"tags": []any{"one", "two"},
		"meta": map[string]any{"kind": "incident"},
	}
	if err := Validate(schema, valid); err != nil {
		t.Fatalf("Validate rejected valid nested payload: %v", err)
	}

	invalid := map[string]any{
		"name": "alice",
		"tags": []any{"one", 2},
		"meta": map[string]any{"kind": "unknown"},
	}
	err := Validate(schema, invalid)
	if err == nil {
		t.Fatal("Validate accepted invalid nested payload")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema validation error, got %v", err)
	}
}

func TestLoadRejectsInvalidJSONSchemaFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, []byte(`{"type":"object","properties":"bad"}`), 0o600); err != nil {
		t.Fatalf("write schema file failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid schema file to fail")
	}
	if !strings.Contains(err.Error(), "invalid JSON schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrettyValidateResponse(t *testing.T) {
	schema, err := DSLToJSONSchema("name, age:int")
	if err != nil {
		t.Fatalf("DSLToJSONSchema failed: %v", err)
	}

	pretty, err := PrettyValidateResponse(schema, `{"name":"alice","age":42}`)
	if err != nil {
		t.Fatalf("PrettyValidateResponse failed: %v", err)
	}
	if pretty == "" || pretty[0] != '{' {
		t.Fatalf("expected pretty JSON output")
	}
}

func TestPrettyValidateResponseRejectsNestedSchemaViolations(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"meta": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{"type": "integer"},
				},
				"required": []any{"count"},
			},
		},
		"required": []any{"meta"},
	}

	_, err := PrettyValidateResponse(schema, `{"meta":{"count":"two"}}`)
	if err == nil {
		t.Fatal("expected nested schema violation")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}
