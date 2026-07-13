package template

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

// Grammar (informal):
//
//	template       := (text | placeholder)*
//	placeholder    := "{{" ws ref ws ("|" ws "default" ws stringlit ws)? "}}"
//	ref            := identifier ("." key)?
//	key            := segment ("." segment)*
//	segment        := [A-Za-z0-9_-]+
//	identifier     := [A-Za-z_][A-Za-z0-9_]*
//	stringlit      := '"' ( '\\' . | [^"\\] )* '"'
//	ws             := ( ' ' | '\t' )*
//
// Backslash escapes inside stringlit: \\ \" \n \t \r — anything else is a
// literal next character. Escaped quotes and pipes inside defaults are
// supported (e.g. `default "a\"b"`, `default "a|b"`).

// Parsed is a pre-tokenized template. Building it once and reusing across
// requests avoids re-parsing per render — useful for hot loops, but also
// what makes load-time Validate possible.
type Parsed struct {
	chunks []chunk
	src    string
}

// chunk is one node in a parsed template.
type chunk interface {
	render(scope Scope) (string, error)
}

type textChunk string

func (t textChunk) render(_ Scope) (string, error) { return string(t), nil }

type refChunk struct {
	scope      string
	key        string // "" when ref is just `{{ scope }}`
	hasDefault bool
	defaultV   string
	raw        string // original text for error messages
}

func (r refChunk) render(scope Scope) (string, error) {
	if r.scope == "cursor" && r.key != "" {
		return "", fmt.Errorf("%w: cursor scope takes no sub-key (in %q)",
			errs.ErrTemplateSyntax, r.raw)
	}
	v, ok := scope.lookup(r.scope, r.key)
	if ok {
		return v, nil
	}
	if r.hasDefault {
		return r.defaultV, nil
	}
	return "", fmt.Errorf("%w: %s.%s (in %q)",
		errs.ErrTemplateMissingKey, r.scope, r.key, r.raw)
}

// Parse tokenizes s into a reusable Parsed. Returns the first syntax error;
// use Validate (after Parse succeeds) for full semantic checks.
func Parse(s string) (Parsed, error) {
	p := parser{src: s}
	return p.parseAll()
}

// Render is the convenience wrapper: parse-then-render in one shot. Hot paths
// should call Parse once and reuse the Parsed.
func (p Parsed) Render(scope Scope) (string, error) {
	if len(p.chunks) == 0 {
		return p.src, nil
	}
	if len(p.chunks) == 1 {
		// Common case: pure-text template — no allocation.
		if t, ok := p.chunks[0].(textChunk); ok {
			return string(t), nil
		}
	}
	var b strings.Builder
	b.Grow(len(p.src))
	for _, c := range p.chunks {
		v, err := c.render(scope)
		if err != nil {
			return "", err
		}
		b.WriteString(v)
	}
	return b.String(), nil
}

// Validate enforces semantic rules and returns ALL issues at once, wrapped in
// errs.ManifestErrors. allowedScopes lists the scope names accepted in this
// context (typically {"config","parent","state","env","cursor"}; subsets are
// fine when a context disallows certain scopes).
func (p Parsed) Validate(allowedScopes []string) error {
	allowed := make(map[string]struct{}, len(allowedScopes))
	for _, s := range allowedScopes {
		allowed[s] = struct{}{}
	}
	var agg errs.ManifestErrors
	for _, c := range p.chunks {
		ref, ok := c.(refChunk)
		if !ok {
			continue
		}
		if _, ok := allowed[ref.scope]; !ok {
			_ = agg.Addf(ref.raw,
				"unknown scope %q (allowed: %v)", ref.scope, allowedScopes)
			continue
		}
		if ref.scope == "cursor" && ref.key != "" {
			_ = agg.Addf(ref.raw,
				"cursor scope takes no sub-key (got %q)", ref.key)
		}
	}
	return agg.AsError()
}

// Validate parses s and runs semantic checks. Convenience for one-shot
// validation; returns nil when the template is well-formed.
func Validate(s string, allowedScopes []string) error {
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	return parsed.Validate(allowedScopes)
}

// =====================================================================
// Parser
// =====================================================================

type parser struct {
	src string
	pos int
}

func (p *parser) parseAll() (Parsed, error) {
	var chunks []chunk
	for p.pos < len(p.src) {
		idx := strings.Index(p.src[p.pos:], "{{")
		if idx < 0 {
			chunks = append(chunks, textChunk(p.src[p.pos:]))
			p.pos = len(p.src)
			break
		}
		if idx > 0 {
			chunks = append(chunks, textChunk(p.src[p.pos:p.pos+idx]))
			p.pos += idx
		}
		ref, err := p.parsePlaceholder()
		if err != nil {
			return Parsed{}, err
		}
		chunks = append(chunks, ref)
	}
	return Parsed{chunks: chunks, src: p.src}, nil
}

func (p *parser) parsePlaceholder() (refChunk, error) {
	start := p.pos
	p.pos += 2 // consume "{{"
	p.skipWS()

	scope := p.readIdent()
	if scope == "" {
		return refChunk{}, p.errf(start, "expected scope name after `{{`")
	}

	var key string
	if p.peek() == '.' {
		p.pos++ // consume '.'
		key = p.readKey()
		if key == "" {
			return refChunk{}, p.errf(start, "expected key after `.`")
		}
	}
	p.skipWS()

	var hasDefault bool
	var defaultV string
	if p.peek() == '|' {
		p.pos++
		p.skipWS()
		fname := p.readIdent()
		if fname != "default" {
			return refChunk{}, p.errf(start,
				"unknown filter %q (only `default` is supported)", fname)
		}
		p.skipWS()
		if p.peek() != '"' {
			return refChunk{}, p.errf(start,
				"`default` filter requires a quoted string")
		}
		s, err := p.readString(start)
		if err != nil {
			return refChunk{}, err
		}
		hasDefault = true
		defaultV = s
		p.skipWS()
	}

	if !p.consume("}}") {
		return refChunk{}, p.errf(start, "expected `}}` to close placeholder")
	}
	end := p.pos
	return refChunk{
		scope:      scope,
		key:        key,
		hasDefault: hasDefault,
		defaultV:   defaultV,
		raw:        p.src[start:end],
	}, nil
}

// readString consumes a "..." literal starting at the current `"`. Supports
// \", \\, \n, \t, \r escapes. Any other `\X` is rendered as the literal X
// (consistent with permissive shell-like quoting).
func (p *parser) readString(placeholderStart int) (string, error) {
	if p.peek() != '"' {
		return "", p.errf(placeholderStart, "expected `\"`")
	}
	p.pos++ // consume opening quote
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '"' {
			p.pos++ // consume closing quote
			return b.String(), nil
		}
		if c == '\\' && p.pos+1 < len(p.src) {
			next := p.src[p.pos+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(next)
			}
			p.pos += 2
			continue
		}
		b.WriteByte(c)
		p.pos++
	}
	return "", p.errf(placeholderStart, "unterminated string literal")
}

// readIdent reads [A-Za-z_][A-Za-z0-9_]*
func (p *parser) readIdent() string {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if isIdentChar(c, p.pos == start) {
			p.pos++
			continue
		}
		break
	}
	return p.src[start:p.pos]
}

// readKey reads dot-separated segments [A-Za-z0-9_-.]+
func (p *parser) readKey() string {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if isKeyChar(c) {
			p.pos++
			continue
		}
		break
	}
	return p.src[start:p.pos]
}

func (p *parser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) consume(s string) bool {
	if p.pos+len(s) > len(p.src) {
		return false
	}
	if p.src[p.pos:p.pos+len(s)] != s {
		return false
	}
	p.pos += len(s)
	return true
}

func (p *parser) skipWS() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.pos++
			continue
		}
		break
	}
}

// errf builds a syntax error with positional context. start is the index of
// the opening `{{` so the message can show the failing placeholder.
func (p *parser) errf(start int, format string, args ...any) error {
	end := p.findPlaceholderEnd(start)
	snippet := p.src[start:end]
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%w: %s (at byte %d, in %q)",
		errs.ErrTemplateSyntax, msg, p.pos, snippet)
}

// findPlaceholderEnd looks ahead for `}}` so error messages can include the
// failing placeholder. Falls back to current position if no `}}` is found.
func (p *parser) findPlaceholderEnd(start int) int {
	if i := strings.Index(p.src[start:], "}}"); i >= 0 {
		return start + i + 2
	}
	if p.pos > start {
		return p.pos
	}
	return min(start+20, len(p.src))
}

func isIdentChar(c byte, first bool) bool {
	switch {
	case c == '_':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case !first && c >= '0' && c <= '9':
		return true
	}
	return false
}

func isKeyChar(c byte) bool {
	switch {
	case c == '_', c == '-', c == '.':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	}
	return false
}
