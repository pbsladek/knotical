package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

func Load(spec string) (map[string]any, error) {
	var schema map[string]any
	if spec == "" {
		return nil, nil
	}
	if _, err := os.Stat(spec); err == nil {
		payload, err := os.ReadFile(spec)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &schema); err != nil {
			return nil, err
		}
	} else {
		var err error
		schema, err = DSLToJSONSchema(spec)
		if err != nil {
			return nil, err
		}
	}
	if err := validateSchemaDocument(schema); err != nil {
		return nil, err
	}
	return schema, nil
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
	if err := validateSchemaDocument(schema); err != nil {
		return err
	}
	return validateValue(schema, value)
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

func validateSchemaDocument(schema map[string]any) error {
	if len(schema) == 0 {
		return fmt.Errorf("schema cannot be empty")
	}
	loader := gojsonschema.NewGoLoader(schema)
	if _, err := gojsonschema.NewSchema(loader); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}
	return nil
}

func validateValue(schema map[string]any, value any) error {
	result, err := gojsonschema.Validate(
		gojsonschema.NewGoLoader(schema),
		gojsonschema.NewGoLoader(value),
	)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	if result.Valid() {
		return nil
	}
	var details bytes.Buffer
	for idx, schemaErr := range result.Errors() {
		if idx > 0 {
			details.WriteString("; ")
		}
		details.WriteString(schemaErr.String())
	}
	return fmt.Errorf("response did not match schema: %s", details.String())
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
