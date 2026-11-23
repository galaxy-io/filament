package httpapi

import "github.com/galaxy-io/filament/rowmodel"

// record is the HTTP connector's private JSON handoff. The connector keeps
// response handling byte-oriented; Source converts records into typed Arrow
// rows at the engine boundary.
type record struct {
	Resource string
	ID       string
	Op       rowmodel.Operation
	Data     []byte
	Key      []string
	// Stream identifies the source event of a row read from a stream.
	Stream *rowmodel.StreamMeta
}

type recordSink interface {
	Push(record) error
	PushBatch([]record) error
}
