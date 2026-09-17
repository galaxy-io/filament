package filament_test

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/streamproof"
	"github.com/galaxy-io/filament/streamkit"
)

func TestEpochCommitValidatesCompletion(t *testing.T) {
	ref := filament.EpochRef{Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}
	domain := filament.DomainKey{Domain: "log", Incarnation: "source"}
	positions := filament.DomainPositions{domain: {Codec: "opaque", Version: 0, Value: []byte("10")}}
	codecs := &streamkit.Registry{}
	if err := codecs.Register("opaque", 0, streamkit.OpaqueCodec{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid", "missing", "wrong epoch", "expanded coverage", "wrong totals", "wrong resource"} {
		t.Run(name, func(t *testing.T) {
			req := filament.EpochCommit{
				Certificate: filament.EpochCertificate{Ref: ref, Coverage: filament.Coverage{Positions: positions.Clone()}, Records: 1, Bytes: 2, Receipts: []filament.EpochReceipt{{Resource: "events", Rows: 1, Bytes: 2}}},
				Completion:  streamproof.New(ref.Binding(), positions, 1, 2, map[string]streamproof.ResourceTotals{"events": {Rows: 1, Bytes: 2}}),
			}
			switch name {
			case "missing":
				req.Completion = filament.EpochCompletion{}
			case "wrong epoch":
				req.Certificate.Ref.Epoch++
			case "expanded coverage":
				req.Certificate.Coverage.Positions[domain] = filament.Position{Codec: "opaque", Version: 0, Value: []byte("11")}
			case "wrong totals":
				req.Certificate.Records++
			case "wrong resource":
				req.Certificate.Receipts[0].Resource = "other"
			}
			err := req.ValidateCompletion(codecs)
			if name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, filament.ErrIncompleteCoverage) {
				t.Fatalf("expected incomplete coverage, got %v", err)
			}
		})
	}
}
