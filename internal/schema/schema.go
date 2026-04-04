package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func Load(spec string) (map[string]any, error) {
	if spec == "" {
		return nil, nil
	}
	if _, err := os.Stat(spec); err == nil {
		payload, err := os.ReadFile(spec)
		if err != nil {
			return nil, err
		}
		var schema map[string]any
		if err := json.Unmarshal(payload, &schema); err != nil {
			return nil, err
		}
		return schema, nil
	}
	return DSLToJSONSchema(spec)
}

func DSLToJSONSchema(dsl string) (map[string]any, error) {
	properties := map[string]any{}
	required := []string{}
	for _, field := range strings.Split(dsl, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		name := field
		typ := "string"
		if left, right, ok := strings.Cut(field, ":"); ok {
			name = strings.TrimSpace(left)
			typ = strings.TrimSpace(right)
		}
		if name == "" {
			return nil, fmt.Errorf("schema field names cannot be empty")
		}
		normalized, err := normalizeType(typ)
		if err != nil {
			return nil, err
		}
		properties[name] = map[string]any{"type": normalized}
		required = append(required, name)
	}
	if len(properties) == 0 {
		return nil, fmt.Errorf("schema DSL must include at least one field")
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}, nil
}

func Validate(schema map[string]any, value any) error {
	typ, _ := schema["type"].(string)
	if typ == "" {
		typ = "object"
	}
	if err := validateType(typ, value); err != nil {
		return fmt.Errorf("top-level schema mismatch: %w", err)
	}
	if typ != "object" {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("JSON value must be an object")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("schema is missing properties")
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			name, _ := item.(string)
			if _, exists := object[name]; !exists {
				return fmt.Errorf("missing required field %q", name)
			}
		}
	}
	if required, ok := schema["required"].([]string); ok {
		for _, name := range required {
			if _, exists := object[name]; !exists {
				return fmt.Errorf("missing required field %q", name)
			}
		}
	}
	for name, rawFieldSchema := range properties {
		fieldSchema, ok := rawFieldSchema.(map[string]any)
		if !ok {
			continue
		}
		value, exists := object[name]
		if !exists {
			continue
		}
		fieldType, _ := fieldSchema["type"].(string)
		if fieldType == "" {
			fieldType = "string"
		}
		if err := validateType(fieldType, value); err != nil {
			return fmt.Errorf("field %q does not match type %q", name, fieldType)
		}
	}
	return nil
}

func PrettyValidateResponse(schema map[string]any, response string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(response), &value); err != nil {
		return "", fmt.Errorf("response was not valid JSON: %w", err)
	}
	if err := Validate(schema, value); err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func normalizeType(typ string) (string, error) {
	switch typ {
	case "", "string", "str":
		return "string", nil
	case "int", "integer":
		return "integer", nil
	case "float", "number":
		return "number", nil
	case "bool", "boolean":
		return "boolean", nil
	case "array", "list":
		return "array", nil
	case "object", "json":
		return "object", nil
	default:
		return "", fmt.Errorf("unsupported schema type %q", typ)
	}
}

func validateType(expected string, value any) error {
	switch expected {
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "integer":
		switch v := value.(type) {
		case float64:
			if v == float64(int64(v)) {
				return nil
			}
		case int, int64:
			return nil
		}
	case "number":
		switch value.(type) {
		case float64, int, int64:
			return nil
		}
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "array":
		if _, ok := value.([]any); ok {
			return nil
		}
	case "object":
		if _, ok := value.(map[string]any); ok {
			return nil
		}
	default:
		return fmt.Errorf("unsupported schema type %q", expected)
	}
	return fmt.Errorf("expected %s", expected)
}
