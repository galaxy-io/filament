package events

import (
	"encoding/json"
	"fmt"
	"time"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
)

// wireEvent is the self-describing JSON frame a Fact crosses transports as.
type wireEvent struct {
	Type     string          `json:"type"`
	Tenant   string          `json:"tenant"`
	Run      string          `json:"run"`
	Resource string          `json:"resource,omitempty"`
	Seq      uint64          `json:"seq"`
	At       time.Time       `json:"at"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// Marshal frames one Fact for the wire.
func Marshal(f Fact) ([]byte, error) {
	raw, err := json.Marshal(f.Data)
	if err != nil {
		return nil, fmt.Errorf("events: marshal %s payload: %w", f.Name, err)
	}
	return json.Marshal(wireEvent{
		Type:     f.Name,
		Tenant:   string(f.Tenant),
		Run:      string(f.Run),
		Resource: f.Resource,
		Seq:      f.Seq,
		At:       f.At,
		Data:     raw,
	})
}

// Unmarshal decodes one wire frame back into a Fact, its payload typed by the
// registry. A malformed frame or unregistered type is an error.
func Unmarshal(b []byte) (Fact, error) {
	var w wireEvent
	if err := json.Unmarshal(b, &w); err != nil {
		return Fact{}, fmt.Errorf("events: unmarshal frame: %w", err)
	}
	def, ok := Lookup(w.Type)
	if !ok {
		return Fact{}, fmt.Errorf("events: unknown event type %q", w.Type)
	}
	data, err := def.decode(w.Data)
	if err != nil {
		return Fact{}, fmt.Errorf("events: decode %s payload: %w", w.Type, err)
	}
	return Fact{
		Envelope: Envelope{
			Tenant:   ingestion.TenantID(w.Tenant),
			Run:      ingestion.RunID(w.Run),
			Resource: w.Resource,
			Seq:      w.Seq,
			At:       w.At,
		},
		Name: w.Type,
		Data: data,
	}, nil
}

// Codec frames Facts for transport buses (see eventbus.Codec); the in-process
// bus never uses it — Facts pass by reference. Transports terminate messages
// this codec cannot decode, so poison frames never block a consumer.
var Codec eventbus.Codec = codec{}

type codec struct{}

func (codec) Encode(payload any) ([]byte, error) {
	f, ok := payload.(Fact)
	if !ok {
		return nil, fmt.Errorf("events: codec expected Fact, got %T", payload)
	}
	return Marshal(f)
}

func (codec) Decode(data []byte) (any, error) {
	f, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Decode extracts the Fact from a delivered message — the by-reference value
// in-process, or the codec-decoded one on a transport.
func Decode(msg eventbus.Message) (Fact, error) {
	f, ok := msg.Payload().(Fact)
	if !ok {
		return Fact{}, fmt.Errorf("events: unexpected payload type %T", msg.Payload())
	}
	return f, nil
}
