package schema

import "testing"

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
