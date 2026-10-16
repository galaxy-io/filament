package embedding

// Reserved payload field names. Sinks write a document's identity, vector and
// text under these names, so metadata can never overwrite them.
const (
	FieldID       = "id"
	FieldVector   = "vector"
	FieldDocument = "document"
	FieldSourceID = "_source_id"
)

// MetadataPrefix is prepended to a metadata key that collides with a reserved
// field. An underscore is used rather than a dot because several stores read
// dots in payload keys as nested paths when filtering.
const MetadataPrefix = "_meta_"

func isReserved(key string) bool {
	switch key {
	case FieldID, FieldVector, FieldDocument, FieldSourceID:
		return true
	default:
		return false
	}
}

// Payload returns the fields to store alongside a document's vector: its
// metadata, its text under FieldDocument and its source key under
// FieldSourceID. It never contains FieldID or FieldVector, which sinks set
// themselves. A metadata key that collides with a reserved field is kept under
// MetadataPrefix+key, prefixed again until it no longer collides, so no column
// is silently dropped.
func Payload(doc Document) map[string]any {
	out := make(map[string]any, len(doc.Metadata)+2)
	out[FieldSourceID] = doc.ID
	if doc.Text != "" {
		out[FieldDocument] = doc.Text
	}
	for key, value := range doc.Metadata {
		name := key
		for isReserved(name) || (name != key && hasKey(doc.Metadata, name)) {
			name = MetadataPrefix + name
		}
		out[name] = value
	}
	return out
}

func hasKey(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}
