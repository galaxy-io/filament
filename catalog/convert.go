// Conversions between the catalog.v1 wire format and the Go types both sides
// of the service share.
package catalog

import (
	"encoding/json"
	"errors"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	catalogv1 "github.com/galaxy-io/filament/api/catalog/v1"
)

// failure turns a verdict field back into an error. Empty is success.
func failure(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}

func encodeConfig(cfg filament.Config) ([]byte, error) {
	if cfg == nil {
		return nil, nil
	}
	return json.Marshal(cfg.Raw())
}

// decodeConfig reads a config_json field. Empty is an empty config.
func decodeConfig(raw []byte) (filament.Config, error) {
	var values map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	return filament.NewConfig(values), nil
}

func decodeSource(connector *catalogv1.Connector) (sourceEntry, error) {
	entry := sourceEntry{contracts: filament.SourceContracts{PlansStreams: connector.GetPlansStreams(), Streams: connector.GetStreams()}}
	err := json.Unmarshal(connector.GetSpecJson(), &entry.spec)
	return entry, err
}

func decodeSink(connector *catalogv1.Connector) (sinkEntry, error) {
	entry := sinkEntry{contracts: filament.SinkContracts{Streams: connector.GetStreams()}}
	err := json.Unmarshal(connector.GetSpecJson(), &entry.spec)
	return entry, err
}

func kindToProto(kind filament.ConnectorKind) catalogv1.Kind {
	switch kind {
	case filament.ConnectorKindSource:
		return catalogv1.Kind_KIND_SOURCE
	case filament.ConnectorKindSink:
		return catalogv1.Kind_KIND_SINK
	default:
		return catalogv1.Kind_KIND_UNSPECIFIED
	}
}

func kindFromProto(kind catalogv1.Kind) filament.ConnectorKind {
	switch kind {
	case catalogv1.Kind_KIND_SOURCE:
		return filament.ConnectorKindSource
	case catalogv1.Kind_KIND_SINK:
		return filament.ConnectorKindSink
	default:
		return filament.ConnectorKindUnspecified
	}
}
