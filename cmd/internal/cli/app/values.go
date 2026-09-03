package app

import (
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func applyConfigPatch(current map[string]any, patch ConfigPatch) map[string]any {
	result := model.CloneConfig(current)
	for name, value := range patch.Values {
		result[name] = model.CloneConfigValue(value)
	}
	for _, name := range patch.Unset {
		delete(result, name)
	}
	return result
}

func emptyPipelineRequest(request SavePipelineRequest) bool {
	return request.Source == "" && request.Sink == "" && request.Resources == nil &&
		request.SyncMode == "" && request.WriteMode == "" &&
		len(request.SourceConfig.Values) == 0 && len(request.SourceConfig.Unset) == 0 &&
		len(request.SinkConfig.Values) == 0 && len(request.SinkConfig.Unset) == 0
}

func connectionMap(kind string, document model.Document) (map[string]model.Connection, error) {
	switch kind {
	case "source":
		return document.Sources, nil
	case "sink":
		return document.Sinks, nil
	default:
		return nil, fmt.Errorf("unknown connection kind %q", kind)
	}
}

func cloneMap[V any](source map[string]V) map[string]V {
	result := make(map[string]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
