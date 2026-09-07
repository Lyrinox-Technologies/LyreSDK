// Package wire provides custom message type registration and handling.
package wire

import (
	"bytes"
	"fmt"

	"github.com/LyrinoxTechnologies/ridged-proto/rdgproto"
)

// DynamicPayload represents a dynamically-defined payload based on config.
type DynamicPayload struct {
	Fields map[string]interface{}
	Schema *PayloadSchema
}

// PayloadSchema defines the structure of a dynamic payload.
type PayloadSchema struct {
	Name   string
	Fields []FieldSchema
}

// FieldSchema defines a field in a dynamic payload.
type FieldSchema struct {
	Name     string
	Type     string // "string", "bytes", "uint32", "uint64", "bool"
	Required bool
}

// Marshal serializes the dynamic payload to binary.
func (p *DynamicPayload) Marshal() ([]byte, error) {
	if p.Schema == nil {
		return nil, fmt.Errorf("payload schema not set")
	}

	buf := new(bytes.Buffer)

	for _, field := range p.Schema.Fields {
		val, exists := p.Fields[field.Name]
		if !exists {
			if field.Required {
				return nil, fmt.Errorf("required field %s is missing", field.Name)
			}
			// Write zero value for optional fields
			val = zeroValue(field.Type)
		}

		if err := writeField(buf, field.Type, val); err != nil {
			return nil, fmt.Errorf("failed to write field %s: %w", field.Name, err)
		}
	}

	return buf.Bytes(), nil
}

// Unmarshal deserializes binary data into the dynamic payload.
func (p *DynamicPayload) Unmarshal(data []byte) error {
	if p.Schema == nil {
		return fmt.Errorf("payload schema not set")
	}

	r := bytes.NewReader(data)
	p.Fields = make(map[string]interface{})

	for _, field := range p.Schema.Fields {
		val, err := readField(r, field.Type)
		if err != nil {
			if field.Required {
				return fmt.Errorf("failed to read required field %s: %w", field.Name, err)
			}
			// Set zero value for optional fields on error
			p.Fields[field.Name] = zeroValue(field.Type)
			continue
		}
		p.Fields[field.Name] = val
	}

	return nil
}

// writeField writes a field value to the buffer based on its type.
func writeField(buf *bytes.Buffer, fieldType string, val interface{}) error {
	switch fieldType {
	case "string":
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
		rdgproto.WriteString(buf, s)

	case "bytes":
		b, ok := val.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte, got %T", val)
		}
		rdgproto.WriteBytes(buf, b)

	case "uint32":
		var n uint32
		switch v := val.(type) {
		case uint32:
			n = v
		case int:
			n = uint32(v)
		case int64:
			n = uint32(v)
		case float64:
			n = uint32(v)
		default:
			return fmt.Errorf("expected uint32, got %T", val)
		}
		rdgproto.WriteUint32(buf, n)

	case "uint64":
		var n uint64
		switch v := val.(type) {
		case uint64:
			n = v
		case int:
			n = uint64(v)
		case int64:
			n = uint64(v)
		case float64:
			n = uint64(v)
		default:
			return fmt.Errorf("expected uint64, got %T", val)
		}
		rdgproto.WriteUint64(buf, n)

	case "bool":
		b, ok := val.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", val)
		}
		rdgproto.WriteBool(buf, b)

	default:
		return fmt.Errorf("unknown field type: %s", fieldType)
	}

	return nil
}

// readField reads a field value from the reader based on its type.
func readField(r *bytes.Reader, fieldType string) (interface{}, error) {
	switch fieldType {
	case "string":
		return rdgproto.ReadString(r)

	case "bytes":
		return rdgproto.ReadBytes(r)

	case "uint32":
		return rdgproto.ReadUint32(r)

	case "uint64":
		return rdgproto.ReadUint64(r)

	case "bool":
		return rdgproto.ReadBool(r)

	default:
		return nil, fmt.Errorf("unknown field type: %s", fieldType)
	}
}

// zeroValue returns the zero value for a field type.
func zeroValue(fieldType string) interface{} {
	switch fieldType {
	case "string":
		return ""
	case "bytes":
		return []byte{}
	case "uint32":
		return uint32(0)
	case "uint64":
		return uint64(0)
	case "bool":
		return false
	default:
		return nil
	}
}

// CustomMessageRegistry manages custom message types.
type CustomMessageRegistry struct {
	schemas map[byte]*PayloadSchema
}

// NewCustomMessageRegistry creates a new registry.
func NewCustomMessageRegistry() *CustomMessageRegistry {
	return &CustomMessageRegistry{
		schemas: make(map[byte]*PayloadSchema),
	}
}

// Register registers a custom message type with its schema.
func (r *CustomMessageRegistry) Register(msgType byte, schema *PayloadSchema) error {
	if msgType < CustomMessageTypeStart || msgType > CustomMessageTypeEnd {
		return fmt.Errorf("message type %d is outside custom range (%d-%d)",
			msgType, CustomMessageTypeStart, CustomMessageTypeEnd)
	}

	if rdgproto.HasPayloadType(msgType) {
		return fmt.Errorf("message type %d is already registered", msgType)
	}

	r.schemas[msgType] = schema

	// Register with rdgproto
	rdgproto.RegisterPayloadType(msgType, func() rdgproto.PayloadUnmarshaler {
		return &DynamicPayload{Schema: schema}
	})

	return nil
}

// Unregister removes a custom message type.
func (r *CustomMessageRegistry) Unregister(msgType byte) {
	delete(r.schemas, msgType)
	rdgproto.UnregisterPayloadType(msgType)
}

// GetSchema returns the schema for a message type.
func (r *CustomMessageRegistry) GetSchema(msgType byte) (*PayloadSchema, bool) {
	schema, exists := r.schemas[msgType]
	return schema, exists
}

// CreatePayload creates a new dynamic payload with the given schema.
func (r *CustomMessageRegistry) CreatePayload(msgType byte, fields map[string]interface{}) (*DynamicPayload, error) {
	schema, exists := r.schemas[msgType]
	if !exists {
		return nil, fmt.Errorf("message type %d not registered", msgType)
	}

	return &DynamicPayload{
		Fields: fields,
		Schema: schema,
	}, nil
}

// ValidatePayload validates a payload against its schema.
func (r *CustomMessageRegistry) ValidatePayload(msgType byte, fields map[string]interface{}) error {
	schema, exists := r.schemas[msgType]
	if !exists {
		return fmt.Errorf("message type %d not registered", msgType)
	}

	for _, field := range schema.Fields {
		val, exists := fields[field.Name]
		if !exists {
			if field.Required {
				return fmt.Errorf("required field %s is missing", field.Name)
			}
			continue
		}

		if err := validateFieldType(field.Type, val); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}

	return nil
}

// validateFieldType checks if a value matches the expected type.
func validateFieldType(fieldType string, val interface{}) error {
	switch fieldType {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
	case "bytes":
		if _, ok := val.([]byte); !ok {
			return fmt.Errorf("expected []byte, got %T", val)
		}
	case "uint32":
		switch val.(type) {
		case uint32, int, int64, float64:
		// OK
		default:
			return fmt.Errorf("expected uint32, got %T", val)
		}
	case "uint64":
		switch val.(type) {
		case uint64, int, int64, float64:
		// OK
		default:
			return fmt.Errorf("expected uint64, got %T", val)
		}
	case "bool":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", val)
		}
	default:
		return fmt.Errorf("unknown field type: %s", fieldType)
	}
	return nil
}
