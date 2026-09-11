package filament

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/galaxy-io/filament/rowmodel"
)

// AttemptRef identifies a stream worker and its admitted ownership token.
type AttemptRef struct {
	RunID                 RunID
	ExecutionID, StreamID string
	Generation, Token     int64
}

// Validate requires complete attempt identity and positive generation and token values.
func (a AttemptRef) Validate() error {
	if a.RunID == "" || a.ExecutionID == "" || a.StreamID == "" || a.Generation <= 0 || a.Token <= 0 || !utf8.ValidString(string(a.RunID)) || !utf8.ValidString(a.ExecutionID) || !utf8.ValidString(a.StreamID) {
		return errors.New("epoch: incomplete attempt identity")
	}
	return nil
}

// EpochRef binds a commit cycle and membership revision to an admitted attempt.
type EpochRef struct {
	Attempt                   AttemptRef
	Epoch, MembershipRevision int64
}

// Validate checks epoch identity without verifying live ownership or durability.
func (e EpochRef) Validate() error {
	if err := e.Attempt.Validate(); err != nil {
		return err
	}
	if e.Epoch <= 0 || e.MembershipRevision <= 0 {
		return errors.New("epoch: invalid epoch or membership revision")
	}
	return nil
}

// ValidateApply checks explicit session binding, not destination-side fencing.
func (e EpochRef) ValidateApply(opts ApplyOptions) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if opts.Epoch == nil || *opts.Epoch != e {
		return ErrEpochMismatch
	}
	return nil
}

// EpochKey identifies a certificate across attempts within a stream generation.
type EpochKey struct {
	StreamID          string
	Generation, Epoch int64
}

// Key returns the attempt-independent certificate identity.
func (e EpochRef) Key() EpochKey { return EpochKey{e.Attempt.StreamID, e.Attempt.Generation, e.Epoch} }

// InboxClaimRef identifies an exact claim, never a sequence watermark.
// The enclosing certificate supplies tenant scope. No inbox infrastructure is
// implemented by these value types.
type InboxClaimRef struct {
	RowID, Owner string
	Token        int64
}

// Coverage holds either candidate domain progress or exact inbox claims, never both.
type Coverage struct {
	Positions DomainPositions
	Claims    []InboxClaimRef
}

// ValidateRepresentation checks representation only; it cannot prove contiguous processing,
// transaction completion, claim authority or pipeline durability. Empty coverage
// is rejected until a coordinator defines an explicit idle/no-progress policy.
func (c Coverage) ValidateRepresentation() error {
	if (len(c.Positions) == 0) == (len(c.Claims) == 0) {
		return ErrIncompleteCoverage
	}
	if _, err := c.Positions.Entries(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(c.Claims))
	for _, claim := range c.Claims {
		if claim.RowID == "" || claim.Owner == "" || claim.Token <= 0 || seen[claim.RowID] || !utf8.ValidString(claim.RowID) || !utf8.ValidString(claim.Owner) {
			return ErrIncompleteCoverage
		}
		seen[claim.RowID] = true
	}
	return nil
}

// Clone returns independently owned positions and claims.
func (c Coverage) Clone() Coverage {
	c.Positions = c.Positions.Clone()
	c.Claims = append([]InboxClaimRef(nil), c.Claims...)
	return c
}

// Canonicalize validates the representation, copies it, and normalizes positions and claim order.
func (c Coverage) Canonicalize(r CodecResolver) (Coverage, error) {
	if err := c.ValidateRepresentation(); err != nil {
		return Coverage{}, err
	}
	c = c.Clone()
	for domain, p := range c.Positions {
		v, err := rowmodel.CanonicalPosition(r, p)
		if err != nil {
			return Coverage{}, err
		}
		c.Positions[domain] = v
	}
	slices.SortFunc(c.Claims, func(a, b InboxClaimRef) int { return strings.Compare(a.RowID, b.RowID) })
	return c, nil
}

// NewPositionCoverage validates and takes an independent copy of candidate
// positions. Receiving boundaries must still validate exported fields.
func NewPositionCoverage(positions DomainPositions) (Coverage, error) {
	c := Coverage{Positions: positions}
	if err := c.ValidateRepresentation(); err != nil {
		return Coverage{}, err
	}
	return c.Clone(), nil
}

// NewClaimCoverage validates and copies exact claims; it does not verify live
// claim ownership or settle inbox rows.
func NewClaimCoverage(claims []InboxClaimRef) (Coverage, error) {
	c := Coverage{Claims: claims}
	if err := c.ValidateRepresentation(); err != nil {
		return Coverage{}, err
	}
	return c.Clone(), nil
}

// StreamingSink extends an opened Sink with a native reusable epoch lifecycle.
// CommitEpoch waits for negotiated durability. AbortEpoch discards only tentative
// effects. CloseSession succeeds only when no later destination effects remain
// possible; a timeout or mere client resource release must return an error.
// Calls are serialized initially. No legacy Commit/Abort adaptation is implied.
type StreamingSink interface {
	BeginEpoch(context.Context, EpochRef) error
	CommitEpoch(context.Context, EpochRef) ([]EpochReceipt, error)
	AbortEpoch(context.Context, EpochRef) error
	CloseSession(context.Context) error
}

// ReceiptEvidence is destination-owned evidence, not comparable source progress.
// Nil evidence means absent. A present value must name its format and have a
// nonnegative version selected by that destination format.
type ReceiptEvidence struct {
	Format  string
	Version int
	Payload []byte
}

// Validate checks a present evidence format and version; nil means absent.
func (e *ReceiptEvidence) Validate() error {
	if e == nil {
		return nil
	}
	if e.Format == "" || !utf8.ValidString(e.Format) || e.Version < 0 {
		return errors.New("epoch: invalid receipt evidence")
	}
	return nil
}

// Clone copies evidence bytes and preserves absent evidence and nil-versus-empty payloads.
func (e *ReceiptEvidence) Clone() *ReceiptEvidence {
	if e == nil {
		return nil
	}
	return &ReceiptEvidence{Format: e.Format, Version: e.Version, Payload: bytes.Clone(e.Payload)}
}

// EpochReceipt contains stable aggregate evidence; it is not a dedup guarantee.
// Evidence uses a destination-owned versioned encoding, immutable across retries.
type EpochReceipt struct {
	Resource    string
	Rows, Bytes int64
	WriteCRC    uint32
	Evidence    *ReceiptEvidence
}

// EpochCertificateFormatVersion identifies the canonical certificate format.
const EpochCertificateFormatVersion = 1

// EpochCertificate is immutable input to a future fenced store transaction.
// Structural canonicalization does not seal coverage or authorize its commit.
// PR 06 must bind this content to private source and completed barrier evidence.
type EpochCertificate struct {
	FormatVersion     int
	Tenant            TenantID
	Ref               EpochRef
	PipelineVersionID string
	Coverage          Coverage
	Receipts          []EpochReceipt
	Records, Bytes    int64
}

// Clone returns independently owned coverage and receipt evidence.
func (c EpochCertificate) Clone() EpochCertificate {
	c.Coverage = c.Coverage.Clone()
	c.Receipts = append([]EpochReceipt(nil), c.Receipts...)
	for i := range c.Receipts {
		c.Receipts[i].Evidence = c.Receipts[i].Evidence.Clone()
	}
	return c
}

// CanonicalBytes v1 sorts semantically unordered domains, claims and receipts;
// preserves nil versus empty opaque payloads; excludes datastore timestamps.
// Position canonicalization is codec-owned. Receipt evidence is already encoded
// by the destination and is not interpreted as source progress.
func (c EpochCertificate) CanonicalBytes(r CodecResolver) ([]byte, error) {
	if c.FormatVersion != EpochCertificateFormatVersion || c.Tenant == "" || c.PipelineVersionID == "" || c.Records < 0 || c.Bytes < 0 || !utf8.ValidString(string(c.Tenant)) || !utf8.ValidString(c.PipelineVersionID) {
		return nil, errors.New("epoch: invalid certificate")
	}
	if err := c.Ref.Validate(); err != nil {
		return nil, err
	}
	c = c.Clone()
	var err error
	c.Coverage, err = c.Coverage.Canonicalize(r)
	if err != nil {
		return nil, err
	}
	if c.Receipts == nil {
		c.Receipts = []EpochReceipt{}
	}
	for _, receipt := range c.Receipts {
		if receipt.Resource == "" || receipt.Rows < 0 || receipt.Bytes < 0 || !utf8.ValidString(receipt.Resource) {
			return nil, errors.New("epoch: invalid receipt")
		}
		if err := receipt.Evidence.Validate(); err != nil {
			return nil, err
		}
	}
	// Receipt ordering has no semantic meaning.
	slices.SortFunc(c.Receipts, func(a, b EpochReceipt) int {
		left, _ := json.Marshal(a)
		right, _ := json.Marshal(b)
		return bytes.Compare(left, right)
	})
	return json.Marshal(c)
}

// Digest hashes the canonical immutable certificate content.
func (c EpochCertificate) Digest(r CodecResolver) ([32]byte, error) {
	data, err := c.CanonicalBytes(r)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// CommittedEpoch pairs a persisted certificate with its datastore-assigned commit time.
type CommittedEpoch struct {
	Certificate EpochCertificate
	CommittedAt time.Time
}

// Stream error categories distinguish ownership, epoch binding, progress, and coverage failures.
var (
	ErrEpochMismatch        = errors.New("stream: epoch mismatch")
	ErrFenced               = errors.New("stream: fenced")
	ErrLeaseExpired         = errors.New("stream: lease expired")
	ErrEpochConflict        = errors.New("stream: epoch conflict")
	ErrPositionRegression   = errors.New("stream: position regression")
	ErrPositionIncomparable = rowmodel.ErrPositionIncomparable
	ErrIncompleteCoverage   = errors.New("stream: incomplete or mixed coverage")
	ErrMembershipChanged    = errors.New("stream: membership changed")
)
