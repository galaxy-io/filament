package rowmodel

import "errors"

type ControlKind uint8

const (
	ControlUnspecified ControlKind = iota
	TxnBegin
	TxnEnd
	ProgressBoundary
	MemberActivated
)

// Control is a standalone source boundary, including boundaries with no rows.
// It supplies source evidence, not proof of pipeline completion or durability.
type Control struct {
	Kind               ControlKind
	Domain             DomainKey
	Position           Position
	TxnID              string
	Resource           string
	MembershipRevision int64
}

func (c Control) Clone() Control { c.Position = c.Position.Clone(); return c }
func (c Control) Validate() error {
	if err := c.Domain.Validate(); err != nil {
		return err
	}
	if err := c.Position.Validate(); err != nil {
		return err
	}
	switch c.Kind {
	case TxnBegin, TxnEnd:
		if c.TxnID == "" || c.Resource != "" || c.MembershipRevision != 0 {
			return errors.New("control: invalid transaction payload")
		}
	case ProgressBoundary:
		if c.TxnID != "" || c.Resource != "" || c.MembershipRevision != 0 {
			return errors.New("control: invalid progress payload")
		}
	case MemberActivated:
		if c.TxnID != "" || c.Resource == "" || c.MembershipRevision <= 0 {
			return errors.New("control: invalid membership payload")
		}
	default:
		return errors.New("control: unknown kind")
	}
	return nil
}

// ResourceDomainRef describes a relationship, not an independently writable
// per-resource checkpoint. Multiple resources may share the same log domain.
type ResourceDomainRef struct {
	Resource string
	Domain   DomainKey
}
