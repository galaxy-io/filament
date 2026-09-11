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
	p.Payload = bytes.ToLower(p.Payload)
	return p, nil
}
func (testCodec) Compare(a, b f.Position) (f.PositionOrder, error) {
	if bytes.Equal(a.Payload, b.Payload) {
		return f.PositionEqual, nil
	}
	return f.PositionIncomparable, nil
}
func TestStreamDefaultsAndApplyBinding(t *testing.T) {
	var opts f.RunOptions
	if err := json.Unmarshal([]byte(`{"BatchMaxRows":10}`), &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Execution.Normalize() != f.ExecutionBounded || opts.Execution.ValidateExecution() != nil {
		t.Fatal("legacy options changed")
	}
	if !errors.Is(f.ExecutionContinuous.ValidateExecution(), f.ErrContinuousDisabled) || f.ExecutionMode("future").ValidateExecution() == nil {
		t.Fatal("execution must fail closed")
	}
	e := f.EpochRef{AttemptRef: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 2}, Epoch: 1, MembershipRevision: 1}
	if err := e.ValidateApply(f.ApplyOptions{Epoch: &e}); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []f.EpochRef{
		{AttemptRef: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 2, Token: 2}, Epoch: 1, MembershipRevision: 1},
		{AttemptRef: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1},
	} {
		if !errors.Is(e.ValidateApply(f.ApplyOptions{Epoch: &changed}), f.ErrFenced) {
			t.Fatal("stale epoch accepted")
		}
	}
	if !errors.Is(e.ValidateApply(f.ApplyOptions{}), f.ErrFenced) {
		t.Fatal("unbound write accepted")
	}
}
func TestCanonicalCertificateAndCoverageOwnership(t *testing.T) {
	a, b := f.DomainKey{Incarnation: "i", Domain: "a"}, f.DomainKey{Incarnation: "i", Domain: "b"}
	cert := f.EpochCertificate{FormatVersion: 1, Tenant: "t", PipelineVersionID: "p", Ref: f.EpochRef{AttemptRef: f.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}, Coverage: f.Coverage{Positions: f.DomainPositions{b: {Codec: "c", Version: 1, Payload: []byte("B")}, a: {Codec: "c", Version: 1, Payload: []byte("A")}}}, Receipts: []f.EpochReceipt{{Resource: "b"}, {Resource: "a"}}}
	copy := cert.Clone()
	copy.Coverage.Positions[a].Payload[0] = 'a'
	copy.Coverage.Positions[b].Payload[0] = 'b'
	copy.Receipts[0], copy.Receipts[1] = copy.Receipts[1], copy.Receipts[0]
	x, err := cert.Digest(testCodec{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := copy.Digest(testCodec{})
	if err != nil || x != y {
		t.Fatalf("canonical mismatch: %v", err)
	}
	if string(cert.Coverage.Positions[a].Payload) != "A" {
		t.Fatal("clone mutated original")
	}
	copy.Ref.Token++
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
		if coverage.Validate() == nil {
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
	c := f.Control{Kind: f.ProgressBoundary, Domain: f.DomainKey{Incarnation: "i", Domain: "d"}, Position: f.Position{Codec: "c", Payload: []byte("p")}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	copy := c.Clone()
	copy.Position.Payload[0] = 'x'
	if string(c.Position.Payload) != "p" {
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
