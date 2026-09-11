package filament

import (
	"errors"
	"github.com/galaxy-io/filament/rowmodel"
	"sort"
	"unicode/utf8"
)

// InboxClaimRef identifies an exact claim, never a sequence watermark.
// The enclosing certificate supplies tenant scope. No inbox infrastructure is
// implemented by these value types.
type InboxClaimRef struct {
	RowID, Owner string
	Token        int64
}
type Coverage struct {
	Positions DomainPositions
	Claims    []InboxClaimRef
}

// Validate checks representation only; it cannot prove contiguous processing,
// transaction completion, claim authority or pipeline durability. Empty coverage
// is rejected until a coordinator defines an explicit idle/no-progress policy.
func (c Coverage) Validate() error {
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
func (c Coverage) Clone() Coverage {
	c.Positions = c.Positions.Clone()
	c.Claims = append([]InboxClaimRef(nil), c.Claims...)
	return c
}
func (c Coverage) Canonicalize(r CodecResolver) (Coverage, error) {
	if err := c.Validate(); err != nil {
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
	sort.Slice(c.Claims, func(i, j int) bool { return c.Claims[i].RowID < c.Claims[j].RowID })
	return c, nil
}

var ErrIncompleteCoverage = errors.New("stream: incomplete or mixed coverage")
