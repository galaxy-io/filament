package filament_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/streamproof"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/streamkit"
)

func TestEpochMessageLayoutsAreVersionedAndBoundToCompletion(t *testing.T) {
	r := &streamkit.Registry{}
	if err := r.Register("opaque", 0, streamkit.OpaqueCodec{}); err != nil {
		t.Fatal(err)
	}
	cert := filament.EpochCertificate{FormatVersion: 1, Tenant: "tenant", PipelineVersionID: "version", Ref: filament.EpochRef{Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}, Coverage: filament.Coverage{Positions: filament.DomainPositions{{Domain: "log", Incarnation: "source"}: {Codec: "opaque", Value: []byte("1")}}}}
	legacy, err := cert.CanonicalBytes(r)
	if err != nil || bytes.Contains(legacy, []byte("MessageSchemas")) {
		t.Fatalf("v1 changed: %s %v", legacy, err)
	}
	cert.MessageSchemas = map[string]schema.MessageLayout{"events": {Fields: []rowmodel.Field{{Name: "name", Logical: rowmodel.LogicalString, Nullable: true}}}}
	if _, err := cert.CanonicalBytes(r); err == nil {
		t.Fatal("v1 accepted schemas")
	}
	cert.FormatVersion = filament.EpochMessageSchemaFormatVersion
	if _, err := cert.CanonicalBytes(r); err != nil {
		t.Fatal(err)
	}
	proof := streamproof.NewWithMessageSchemas(cert.Ref.Binding(), cert.Coverage.Positions, 0, 0, nil, cert.MessageSchemas)
	request := filament.EpochCommit{Certificate: cert, Completion: proof}
	if err := request.ValidateCompletion(r); err != nil {
		t.Fatal(err)
	}
	cloned := cert.Clone()
	cloned.MessageSchemas["events"].Fields[0].Name = "changed"
	if cert.MessageSchemas["events"].Fields[0].Name != "name" {
		t.Fatal("clone aliases layouts")
	}
	request.Certificate = cloned
	if err := request.ValidateCompletion(r); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("substituted layout: %v", err)
	}
	cert.MessageSchemas = nil
	if _, err := cert.CanonicalBytes(r); err == nil {
		t.Fatal("v2 accepted missing layout")
	}
}
