package filament_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	f "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// This contract test deliberately has no streamkit dependency: PR 02 stands alone.
type testCodec struct{}

func (testCodec) Lookup(string, int) (rowmodel.PositionCodec, error) { return testCodec{}, nil }
func (testCodec) Validate(p f.Position) error                        { return p.Validate() }
func (testCodec) Canonicalize(p f.Position) (f.Position, error) {
	p.Value = bytes.ToLower(p.Value)
	return p, nil
}
func (testCodec) Compare(a, b f.Position) (f.PositionOrder, error) {
	if bytes.Equal(a.Value, b.Value) {
		return f.PositionEqual, nil
	}
	return f.PositionIncomparable, nil
}
func TestStreamDefaultsAndApplyBinding(t *testing.T) {
	var opts f.RunOptions
	if err := json.Unmarshal([]byte(`{"BatchMaxRows":10}`), &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Execution.Normalize() != f.ExecutionBounded || opts.Execution.Validate() != nil {
		t.Fatal("legacy options changed")
	}
	if f.ExecutionContinuous.Validate() != nil || f.ExecutionMode("future").Validate() == nil {
		t.Fatal("execution must fail closed")
	}
	e := f.EpochRef{Attempt: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 2}, Epoch: 1, MembershipRevision: 1}
	if err := e.ValidateApply(f.ApplyOptions{Epoch: &e}); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []f.EpochRef{
		{Attempt: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 2, Token: 2}, Epoch: 1, MembershipRevision: 1},
		{Attempt: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1},
	} {
		if !errors.Is(e.ValidateApply(f.ApplyOptions{Epoch: &changed}), f.ErrEpochMismatch) {
			t.Fatal("stale epoch accepted")
		}
	}
	if !errors.Is(e.ValidateApply(f.ApplyOptions{}), f.ErrEpochMismatch) {
		t.Fatal("unbound write accepted")
	}
}
func TestCanonicalCertificateAndCoverageOwnership(t *testing.T) {
	a, b := f.DomainKey{Incarnation: "i", Domain: "a"}, f.DomainKey{Incarnation: "i", Domain: "b"}
	cert := f.EpochCertificate{FormatVersion: f.EpochCertificateFormatVersion, Tenant: "t", PipelineVersionID: "p", Ref: f.EpochRef{Attempt: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}, Coverage: f.Coverage{Positions: f.DomainPositions{b: {Codec: "c", Version: 1, Value: []byte("B")}, a: {Codec: "c", Version: 1, Value: []byte("A")}}}, Receipts: []f.EpochReceipt{{Resource: "b"}, {Resource: "a"}}}
	copy := cert.Clone()
	copy.Coverage.Positions[a].Value[0] = 'a'
	copy.Coverage.Positions[b].Value[0] = 'b'
	copy.Receipts[0], copy.Receipts[1] = copy.Receipts[1], copy.Receipts[0]
	x, err := cert.Digest(testCodec{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := copy.Digest(testCodec{})
	if err != nil || x != y {
		t.Fatalf("canonical mismatch: %v", err)
	}
	if string(cert.Coverage.Positions[a].Value) != "A" {
		t.Fatal("clone mutated original")
	}
	copy.Ref.Attempt.Token++
	z, _ := copy.Digest(testCodec{})
	if x == z {
		t.Fatal("authority omitted from digest")
	}
	wire, err := json.Marshal(cert.Coverage.Positions)
	if err != nil {
		t.Fatal(err)
	}
	var positions f.DomainPositions
	if err := json.Unmarshal(wire, &positions); err != nil {
		t.Fatal(err)
	}
	if len(positions) != 2 {
		t.Fatal("position wire roundtrip")
	}
	if err := json.Unmarshal([]byte(`[{"Domain":{"Incarnation":"i","Domain":"a"},"Position":{"Codec":"c"}},{"Domain":{"Incarnation":"i","Domain":"a"},"Position":{"Codec":"c"}}]`), &positions); err == nil {
		t.Fatal("duplicate domain accepted")
	}
	for _, coverage := range []f.Coverage{{}, {Positions: cert.Coverage.Positions, Claims: []f.InboxClaimRef{{RowID: "1", Owner: "o", Token: 1}}}, {Claims: []f.InboxClaimRef{{RowID: "1", Owner: "o", Token: 1}, {RowID: "1", Owner: "o", Token: 2}}}} {
		if coverage.ValidateRepresentation() == nil {
			t.Fatal("invalid coverage accepted")
		}
	}
	claims := f.Coverage{Claims: []f.InboxClaimRef{{RowID: "2", Owner: "o", Token: 1}, {RowID: "1", Owner: "o", Token: 2}}}
	sealed, err := claims.Canonicalize(nil)
	if err != nil || sealed.Claims[0].RowID != "1" || claims.Claims[0].RowID != "2" {
		t.Fatal("claim canonicalization ownership")
	}
	_, err = rowmodel.ComparePositions(testCodec{}, a, cert.Coverage.Positions[a], f.DomainKey{Incarnation: "other", Domain: "a"}, cert.Coverage.Positions[a])
	if !errors.Is(err, f.ErrPositionIncomparable) {
		t.Fatal("incarnation ordering inferred")
	}
}
func TestControlValidationAndOwnership(t *testing.T) {
	c := f.Control{Kind: f.ProgressBoundary, Domain: f.DomainKey{Incarnation: "i", Domain: "d"}, Position: f.Position{Codec: "c", Value: []byte("p")}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	copy := c.Clone()
	copy.Position.Value[0] = 'x'
	if string(c.Position.Value) != "p" {
		t.Fatal("control aliases bytes")
	}
	for _, kind := range []f.ControlKind{f.ControlUnspecified, f.TxnBegin, f.TxnEnd, f.MemberActivated, 99} {
		c.Kind = kind
		if c.Validate() == nil {
			t.Fatalf("invalid control %d", kind)
		}
	}
	c.Kind = f.MemberActivated
	c.Resource = "r"
	c.MembershipRevision = 1
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageConstructorsOwnInputs(t *testing.T) {
	domain := f.DomainKey{Incarnation: "i", Domain: "d"}
	positions := f.DomainPositions{domain: {Codec: "c", Value: []byte("p")}}
	coverage, err := f.NewPositionCoverage(positions)
	if err != nil {
		t.Fatal(err)
	}
	positions[domain].Value[0] = 'x'
	delete(positions, domain)
	if string(coverage.Positions[domain].Value) != "p" {
		t.Fatal("position coverage borrows input")
	}
	claims := []f.InboxClaimRef{{RowID: "1", Owner: "o", Token: 1}}
	claimed, err := f.NewClaimCoverage(claims)
	if err != nil {
		t.Fatal(err)
	}
	claims[0].Token = 2
	if claimed.Claims[0].Token != 1 {
		t.Fatal("claim coverage borrows input")
	}
	if _, err := f.NewPositionCoverage(nil); err == nil {
		t.Fatal("empty position coverage")
	}
	if _, err := f.NewClaimCoverage(nil); err == nil {
		t.Fatal("empty claim coverage")
	}
	coverage.Claims = claimed.Claims
	if coverage.ValidateRepresentation() == nil {
		t.Fatal("mutated mixed coverage accepted")
	}
}

func TestRunStreamAttemptIdentity(t *testing.T) {
	a := f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 2}
	spec := f.RunSpec{StreamAttempt: &a, Run: "r", ExecutionID: "e", ReplicationStream: &f.StreamRef{ID: "s", Generation: 1}, Options: f.RunOptions{Execution: f.ExecutionContinuous}}
	if err := spec.ValidateStreamAttempt(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*f.RunSpec){
		func(s *f.RunSpec) { s.Run = "wrong" },
		func(s *f.RunSpec) { s.ExecutionID = "wrong" },
		func(s *f.RunSpec) { s.ReplicationStream = &f.StreamRef{ID: "wrong", Generation: 1} },
		func(s *f.RunSpec) { s.ReplicationStream = &f.StreamRef{ID: "s", Generation: 2} },
		func(s *f.RunSpec) { s.StreamAttempt = nil },
	} {
		copy := spec
		change(&copy)
		if copy.ValidateStreamAttempt() == nil {
			t.Fatal("conflicting or absent attempt accepted")
		}
	}
	spec.Run = ""
	spec.ExecutionID = ""
	spec.ReplicationStream = nil
	if err := spec.ValidateStreamAttempt(); err != nil {
		t.Fatal("attempt must be authoritative when optional mirrors are absent", err)
	}
	spec.Options.Execution = f.ExecutionBounded
	spec.Run = "unrelated"
	if err := spec.ValidateStreamAttempt(); err != nil {
		t.Fatal("bounded dispatch changed", err)
	}
}

func TestReceiptEvidenceOwnershipAndPresence(t *testing.T) {
	cert := f.EpochCertificate{Receipts: []f.EpochReceipt{{Resource: "r", Evidence: &f.ReceiptEvidence{Format: "receipt", Version: 0, Payload: []byte{1}}}, {Resource: "absent"}}}
	clone := cert.Clone()
	clone.Receipts[0].Evidence.Payload[0] = 2
	clone.Receipts[0].Evidence.Format = "changed"
	if cert.Receipts[0].Evidence.Payload[0] != 1 || cert.Receipts[0].Evidence.Format != "receipt" || clone.Receipts[1].Evidence != nil {
		t.Fatal("receipt evidence ownership/presence lost")
	}
	for _, e := range []*f.ReceiptEvidence{{}, {Format: "receipt", Version: -1}} {
		if e.Validate() == nil {
			t.Fatal("invalid present evidence")
		}
	}
	nilPayload := (&f.ReceiptEvidence{Format: "receipt"}).Clone()
	emptyPayload := (&f.ReceiptEvidence{Format: "receipt", Payload: []byte{}}).Clone()
	if nilPayload.Payload != nil || emptyPayload.Payload == nil {
		t.Fatal("nil/empty evidence payload lost")
	}
}
