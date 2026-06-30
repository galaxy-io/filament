package eventbus

import "strings"

// Route is a portable pattern resolved for one provider: the broker subscription
// target plus the matcher the client applies to whatever the broker over-delivers.
type Route struct {
	// Target is the provider's subscription target derived from the pattern (a
	// NATS subject, a Kafka topic, …). Empty means no native narrowing — match
	// everything via ClientFilter.
	Target string
	// ClientFilter matches what Target cannot express. Always safe to apply; when
	// Exact is true it is redundant and may be skipped.
	ClientFilter Filter
	// Exact reports whether Target alone delivers exactly the matching subjects.
	Exact bool
}

// Resolver turns a portable pattern into a provider-specific [Route], so subject
// semantics stay identical across providers while native routing differs. Every
// [Bus] is a Resolver.
type Resolver interface {
	Resolve(pattern string) (Route, error)
}

// LiteralPrefix returns the leading run of literal tokens — the part before the
// first '*' or '>' — joined by the separator, and whether the whole pattern is
// literal. Flat-topic providers use it to derive the coarsest native target.
func LiteralPrefix(pattern string) (prefix string, exact bool) {
	toks := strings.Split(pattern, Separator)
	n := 0
	for _, t := range toks {
		if t == TokenWildcard || t == TailWildcard {
			break
		}
		n++
	}
	return strings.Join(toks[:n], Separator), n == len(toks)
}

// PassthroughResolver maps a pattern to itself, for transports whose native
// grammar matches eventbus's '*'/'>' (NATS). The broker does all matching.
type PassthroughResolver struct{}

func (PassthroughResolver) Resolve(pattern string) (Route, error) {
	f, err := Compile(pattern)
	if err != nil {
		return Route{}, err
	}
	return Route{Target: pattern, ClientFilter: f, Exact: true}, nil
}

// PrefixTopicResolver maps a pattern onto the coarsest flat topic it can — the
// leading literal run — and leaves the rest to the client, for brokers with no
// hierarchical wildcards (Kafka, SQS). The consumer receives a superset and
// applies ClientFilter.
type PrefixTopicResolver struct{}

func (PrefixTopicResolver) Resolve(pattern string) (Route, error) {
	f, err := Compile(pattern)
	if err != nil {
		return Route{}, err
	}
	target, exact := LiteralPrefix(pattern)
	return Route{Target: target, ClientFilter: f, Exact: exact}, nil
}
