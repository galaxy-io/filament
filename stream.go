package filament

import "github.com/galaxy-io/filament/rowmodel"

type DomainKey = rowmodel.DomainKey
type Position = rowmodel.Position
type DomainPosition = rowmodel.DomainPosition
type DomainPositions = rowmodel.DomainPositions
type PositionOrder = rowmodel.PositionOrder
type PositionCodec = rowmodel.PositionCodec
type CodecResolver = rowmodel.CodecResolver
type Control = rowmodel.Control
type ControlKind = rowmodel.ControlKind
type ResourceDomainRef = rowmodel.ResourceDomainRef

const (
	PositionIncomparable = rowmodel.PositionIncomparable
	PositionEqual        = rowmodel.PositionEqual
	PositionBefore       = rowmodel.PositionBefore
	PositionAfter        = rowmodel.PositionAfter
	ControlUnspecified   = rowmodel.ControlUnspecified
	TxnBegin             = rowmodel.TxnBegin
	TxnEnd               = rowmodel.TxnEnd
	ProgressBoundary     = rowmodel.ProgressBoundary
	MemberActivated      = rowmodel.MemberActivated
)
