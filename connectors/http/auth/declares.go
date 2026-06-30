package auth

// WriteKind enumerates the request locations an Authenticator may modify.
type WriteKind int

const (
	// WriteHeader means the Authenticator sets an HTTP header.
	WriteHeader WriteKind = iota
	// WriteQueryParam means the Authenticator appends or sets a URL query
	// parameter.
	WriteQueryParam
)

func (w WriteKind) String() string {
	switch w {
	case WriteHeader:
		return "header"
	case WriteQueryParam:
		return "query"
	}
	return "?"
}

// Write describes one mutation an Authenticator declares it will perform.
// Used by chainAuth to detect cross-step collisions at Build time, before any
// request is ever sent.
type Write struct {
	Kind WriteKind
	Name string // header name or query parameter name
}

// Declarer is implemented by Authenticators that statically know which headers
// or query params they write. Chain uses these declarations to surface
// configuration mistakes (two steps both setting Authorization, etc.) early
// — at manifest load — instead of letting the second silently overwrite the
// first at request time.
//
// Authenticators that derive their target from runtime template scope cannot
// declare statically; they should not implement this interface and the chain
// will treat them as opaque.
type Declarer interface {
	DeclaredWrites() []Write
}

// declaredWrites is a small helper: returns the writes for a if it implements
// Declarer, else nil.
func declaredWrites(a Authenticator) []Write {
	if d, ok := a.(Declarer); ok {
		return d.DeclaredWrites()
	}
	return nil
}
