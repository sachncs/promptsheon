package eval

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestJSONSchema_TypeString(t *testing.T) {
	schema := json.RawMessage(`{"type": "string"}`)
	cases := []struct {
		name   string
		actual string
		want   bool
	}{
		{"string ok", `"hello"`, true},
		{"number fails", `42`, false},
		{"bool fails", `true`, false},
		{"null fails", `null`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := (JSONSchema{}).ScoreCase(json.RawMessage(c.actual), schema)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestJSONSchema_TypeInteger(t *testing.T) {
	schema := json.RawMessage(`{"type": "integer"}`)
	cases := []struct {
		name   string
		actual string
		want   bool
	}{
		{"int ok", `42`, true},
		{"float with .0 ok", `42.0`, true},
		{"float with .5 fails", `42.5`, false},
		{"string fails", `"42"`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := (JSONSchema{}).ScoreCase(json.RawMessage(c.actual), schema)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestJSONSchema_Required(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "required": ["name", "age"]}`)
	cases := []struct {
		name   string
		actual string
		want   bool
	}{
		{"both present", `{"name":"alice","age":30}`, true},
		{"one missing", `{"name":"alice"}`, false},
		{"both missing", `{}`, false},
		{"non-object fails", `"alice"`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := (JSONSchema{}).ScoreCase(json.RawMessage(c.actual), schema)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestJSONSchema_PropertiesNested(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "integer"},
			"email": {"type": "string"}
		},
		"required": ["id"]
	}`)
	cases := []struct {
		name   string
		actual string
		want   bool
	}{
		{"ok", `{"id": 1, "email": "a@b.c"}`, true},
		{"email wrong type", `{"id": 1, "email": 42}`, false},
		{"id wrong type", `{"id": "1"}`, false},
		{"missing id", `{"email": "a@b.c"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := (JSONSchema{}).ScoreCase(json.RawMessage(c.actual), schema)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestJSONSchema_Enum(t *testing.T) {
	schema := json.RawMessage(`{"enum": ["red", "green", "blue"]}`)
	cases := []struct {
		name   string
		actual string
		want   bool
	}{
		{"ok", `"red"`, true},
		{"other fails", `"yellow"`, false},
		{"non-string fails", `42`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := (JSONSchema{}).ScoreCase(json.RawMessage(c.actual), schema)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestJSONSchema_BadSchema(t *testing.T) {
	_, err := (JSONSchema{}).ScoreCase(json.RawMessage(`"x"`), json.RawMessage(`"not a schema"`))
	if err == nil || !strings.Contains(err.Error(), "schema is not an object") {
		t.Errorf("expected schema-not-object error, got %v", err)
	}
}

func TestJSONSchema_EmptySchema(t *testing.T) {
	_, err := (JSONSchema{}).ScoreCase(json.RawMessage(`"x"`), json.RawMessage(``))
	if err == nil {
		t.Errorf("expected empty-schema error")
	}
}

// TestJSONSchema_RejectsUnsupportedKeywords locks in the SEC-3a
// acceptance: a schema that uses only unsupported keywords
// (allOf, $ref, oneOf) returns ErrUnsupportedSchema from
// ScoreCase.
func TestJSONSchema_RejectsUnsupportedKeywords(t *testing.T) {
	cases := []struct {
		name   string
		schema string
	}{
		{"allOf", `{"allOf": [{"type": "string"}]}`},
		{"$ref", `{"$ref": "#/definitions/foo"}`},
		{"oneOf", `{"oneOf": [{"type": "string"}, {"type": "integer"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := (JSONSchema{}).ScoreCase(json.RawMessage(`"x"`), json.RawMessage(c.schema))
			if !errors.Is(err, ErrUnsupportedSchema) {
				t.Errorf("expected ErrUnsupportedSchema, got %v", err)
			}
		})
	}
}
