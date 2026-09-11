package rowmodel

import (
	"fmt"
	"strings"
)

// Reserved envelope column names identify row-aligned event metadata.
const (
	EventIDField        = "_filament_event_id"
	EventIdentityField  = "_filament_event_identity"
	EventTimestampField = "_filament_event_ts"
	EventHeadersField   = "_filament_headers"
	EventKeyField       = "_filament_key"
	EventPayloadField   = "_filament_payload"
)

func envelopeFields() []Field {
	return []Field{
		{Name: EventIDField, Logical: LogicalString, Nullable: true},
		{Name: EventIdentityField, Logical: LogicalBytes, Nullable: true},
		// RFC3339Nano in UTC preserves sub-microsecond source precision, unlike the
		// existing Arrow timestamp mapping (microseconds).
		{Name: EventTimestampField, Logical: LogicalString, Nullable: true},
		{Name: EventHeadersField, Logical: LogicalBytes, Nullable: true},
		{Name: EventKeyField, Logical: LogicalBytes, Nullable: true},
		{Name: EventPayloadField, Logical: LogicalBytes, Nullable: true},
	}
}

// WithEnvelopeFields declares the SDK-owned suffix. It rejects user collisions
// before adding columns; callers cannot mark arbitrary fields as SDK-generated.
func WithEnvelopeFields(s Schema) (Schema, error) {
	if s.envelope {
		return Schema{}, fmt.Errorf("resource %q already has an envelope", s.Resource)
	}
	if err := ValidateReservedFields(s); err != nil {
		return Schema{}, err
	}
	s = s.Clone()
	s.Fields = append(s.Fields, envelopeFields()...)
	s.envelope = true
	return s, nil
}

// ValidateReservedFields permits only the exact SDK-declared suffix. The marker
// is schema provenance, not a security boundary against malicious Go code.
func ValidateReservedFields(s Schema) error {
	start := len(s.Fields)
	if s.envelope {
		fields := envelopeFields()
		start -= len(fields)
		if start < 0 {
			return fmt.Errorf("resource %q has incomplete envelope fields", s.Resource)
		}
		for i, f := range fields {
			if s.Fields[start+i] != f {
				return fmt.Errorf("resource %q has modified envelope field %q", s.Resource, f.Name)
			}
		}
	}
	for _, f := range s.Fields[:start] {
		if strings.HasPrefix(strings.ToLower(f.Name), "_filament_") {
			return fmt.Errorf("resource %q uses reserved Filament column %q", s.Resource, f.Name)
		}
	}
	return nil
}
