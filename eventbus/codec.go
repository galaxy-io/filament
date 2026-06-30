package eventbus

// Codec marshals payloads for transports that cross a process boundary; the
// in-proc bus passes payloads by reference and never uses one. Decode returns
// the domain type as any — the domain supplies the codec and its adapter
// asserts the result.
type Codec interface {
	Encode(payload any) ([]byte, error)
	Decode(data []byte) (any, error)
}
