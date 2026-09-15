package rowmodel

// EventIdentity identifies one source event independently of its run, attempt,
// generation or consumer. Ordinal distinguishes events sharing a position.
type EventIdentity struct {
	Domain   DomainKey
	Position Position
	Ordinal  uint64
}

// Clone returns an event identity with independently owned position bytes.
func (e EventIdentity) Clone() EventIdentity { e.Position = e.Position.Clone(); return e }

// StreamMeta carries native event metadata. Its presence does not select
// continuous execution: bounded catch-up can also carry event identities.
// It cannot certify coverage or replace the row-aligned envelope columns.
type StreamMeta struct{ Identity EventIdentity }

// Clone copies stream metadata and its position bytes, preserving nil metadata.
func (m *StreamMeta) Clone() *StreamMeta {
	if m == nil {
		return nil
	}
	return &StreamMeta{Identity: m.Identity.Clone()}
}
