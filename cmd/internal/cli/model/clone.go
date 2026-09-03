package model

// CloneConfig deep-copies a config map. The result is never nil so callers
// can write into it.
func CloneConfig(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = CloneConfigValue(value)
	}
	return out
}

// CloneConfigValue deep-copies nested maps and slices; scalars pass through.
func CloneConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneConfig(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = CloneConfigValue(item)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}
