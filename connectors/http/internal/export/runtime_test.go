package export

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

type countingControl struct{ starts int }

func (c *countingControl) Start(context.Context, manifest.Resource, template.Scope) ([]byte, error) {
	c.starts++
	return nil, errors.New("unexpected creation")
}

func (*countingControl) Poll(context.Context, string, manifest.ExportRequest, template.Scope) ([]byte, error) {
	return nil, errors.New("unexpected poll")
}

type rejectingSink struct{ err error }

func (s rejectingSink) Emit(map[string]any, map[string]string, string) (int, error) { return 0, s.err }
func (s rejectingSink) Checkpoint(string, string) error                             { return s.err }
func (rejectingSink) Completed(int)                                                 {}

// A failed intent publication must cross the adapter boundary before any
// creation request. This holds for both legacy full reads and scoped jobs.
func TestCheckpointFailurePreventsCreation(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "parent"}[scoped], func(t *testing.T) {
			res := manifest.Resource{Name: "items", Export: &manifest.ExportSpec{
				Start: manifest.ExportStart{ExportRequest: manifest.ExportRequest{Method: "POST", Path: "/exports"}, Capture: map[string]string{"id": "$.id"}},
				Wait:  manifest.ExportWait{TimeoutSeconds: 30},
			}}
			var parents []map[string]string
			if scoped {
				res.Parent = &manifest.ParentRef{Resource: "segments"}
				res.Export.ParentKey = []string{"id"}
				parents = []map[string]string{{"id": "segment"}}
			}
			control := &countingControl{}
			runtime := &Runtime{Control: control, BaseURL: "https://api.example.com"}
			unavailable := errors.New("checkpoint unavailable")
			if err := runtime.Run(t.Context(), res, rejectingSink{err: unavailable}, parents); !errors.Is(err, unavailable) {
				t.Fatalf("error = %v", err)
			}
			if control.starts != 0 {
				t.Fatal("created job without a published intent")
			}
		})
	}
}
