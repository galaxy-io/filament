package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

type evolvingTestSource struct{ *continuousTestSource }

func (s evolvingTestSource) OpenMessageStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	return s.OpenStream(ctx, opts)
}

type evolvingTestSink struct {
	*continuousTestSink
	policy schema.Policy
}

func (s evolvingTestSink) EnsureSchema(context.Context, string, rowmodel.Schema) error { return nil }

func (s evolvingTestSink) SchemaPolicy() schema.Policy { return s.policy }

func TestMessageSchemaResumeAdmission(t *testing.T) {
	for _, mode := range []string{"restore", "legacy", "disabled", "contract changed", "resource changed"} {
		t.Run(mode, func(t *testing.T) {
			cfg, store, source, sink := continuousFixture(t)
			cfg.Source = evolvingTestSource{source}
			cfg.Sink = evolvingTestSink{sink, schema.NullableAdd}
			store.state.LastEpoch = &filament.EpochKey{StreamID: "stream", Generation: 1, Epoch: 1}
			store.cert = &filament.CommittedEpoch{Certificate: filament.EpochCertificate{FormatVersion: filament.EpochMessageSchemaFormatVersion, PipelineVersionID: cfg.Spec.PipelineVersionID, MessageSchemas: map[string]schema.MessageLayout{"events": {Fields: []rowmodel.Field{{Name: "email", Logical: rowmodel.LogicalString, Nullable: true}}}}}}
			switch mode {
			case "legacy":
				store.cert.Certificate.FormatVersion = 1
				store.cert.Certificate.MessageSchemas = nil
			case "disabled":
				cfg.Sink = evolvingTestSink{sink, ""}
			case "contract changed":
				store.cert.Certificate.PipelineVersionID = "older"
			case "resource changed":
				store.cert.Certificate.MessageSchemas["other"] = store.cert.Certificate.MessageSchemas["events"]
			}
			err := restoreMessageSchemas(context.Background(), &cfg, store.state)
			if mode != "restore" {
				if !errors.Is(err, schema.ErrBlocked) {
					t.Fatalf("unsafe resume admitted: %v", err)
				}
				return
			}
			if err != nil || len(cfg.messageSchemas["events"].Fields) != 1 {
				t.Fatalf("layout not restored: %v", err)
			}
			cfg.messageSchemas["events"].Fields[0].Name = "changed"
			if store.cert.Certificate.MessageSchemas["events"].Fields[0].Name != "email" {
				t.Fatal("restored layout aliases history")
			}
		})
	}
}
