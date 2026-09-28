package filament

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/galaxy-io/filament/internal/streamproof"
)

// EpochCompletion is immutable, process-local evidence from a bound and sealed
// pipeline. It is not proof of sink durability; the native sink supplies receipts.
type EpochCompletion = streamproof.Completion

// EpochCommit binds a certificate to source controls accepted by the pipeline.
// Coverage remains connector-authoritative; offsets alone cannot prove a prefix.
// Completion is required for new commits. Historical identical retries may use
// only Certificate, since their canonical content has already been certified.
type EpochCommit struct {
	Certificate EpochCertificate
	Completion  EpochCompletion
}

// Binding returns a deterministic identity for process-local completion evidence.
func (e EpochRef) Binding() string { data, _ := json.Marshal(e); return string(data) }

// ValidateCompletion checks exact canonical coverage and completed Apply totals.
// No public constructor can manufacture nonzero pipeline completion evidence.
func (c EpochCommit) ValidateCompletion(codecs CodecResolver) error {
	cert := c.Certificate
	if err := cert.Ref.Validate(); err != nil {
		return err
	}
	if len(cert.Coverage.Claims) != 0 || c.Completion.Binding() != cert.Ref.Binding() {
		return ErrIncompleteCoverage
	}
	covered, err := cert.Coverage.Canonicalize(codecs)
	if err != nil {
		return err
	}
	completed, err := (Coverage{Positions: c.Completion.Positions()}).Canonicalize(codecs)
	if err != nil {
		return err
	}
	left, err := json.Marshal(covered)
	if err != nil {
		return err
	}
	right, err := json.Marshal(completed)
	if err != nil {
		return err
	}
	if !bytes.Equal(left, right) {
		return ErrIncompleteCoverage
	}
	rows, nbytes := c.Completion.Totals()
	if cert.Records != rows || cert.Bytes != nbytes {
		return ErrIncompleteCoverage
	}
	var receiptRows, receiptBytes int64
	expected := c.Completion.Resources()
	actual := make(map[string]streamproof.ResourceTotals)
	for _, r := range cert.Receipts {
		if r.Rows < 0 || r.Bytes < 0 || r.Rows > math.MaxInt64-receiptRows || r.Bytes > math.MaxInt64-receiptBytes {
			return ErrIncompleteCoverage
		}
		receiptRows += r.Rows
		receiptBytes += r.Bytes
		total := actual[r.Resource]
		total.Rows += r.Rows
		total.Bytes += r.Bytes
		actual[r.Resource] = total
	}
	if len(actual) != len(expected) {
		return ErrIncompleteCoverage
	}
	for resource, total := range expected {
		if got, ok := actual[resource]; !ok || got != total {
			return ErrIncompleteCoverage
		}
	}
	if receiptRows != rows || receiptBytes != nbytes {
		return ErrIncompleteCoverage
	}
	return nil
}
