// Package transform compiles a yaml definition of column transforms into a
// plan that runs over Arrow record batches. A definition is parsed once,
// type-checked against a resource schema once, and then applied to every
// batch as a fixed list of whole-column operations. Rows are visited only
// inside a kernel's inner loop, never by the plan itself.
package transform

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// supportedVersion is the grammar version this package parses.
const supportedVersion = 1

// Definition is a parsed transform document: a step list per resource.
type Definition struct {
	Version   int                 `yaml:"version"`
	Resources map[string]Resource `yaml:"resources"`
}

// Resource holds the steps applied to one resource, in order.
type Resource struct {
	Steps []Step `yaml:"steps"`
}

// Step is one of rename, drop, or set; the grammar allows exactly one per
// step. Where belongs to set alone and limits which rows the set rewrites.
// No step removes rows. Entries of one set are independent of each other,
// so their order carries no meaning; columns a set creates are appended in
// name order.
type Step struct {
	Rename map[string]string `yaml:"rename,omitempty"`
	Drop   []string          `yaml:"drop,omitempty"`
	Set    map[string]Expr   `yaml:"set,omitempty"`
	Where  *Expr             `yaml:"where,omitempty"`
}

// Expr is a column reference, a literal, or a function call, and exactly one
// of Col, Lit, or Fn is set. Literals are never nil, so Lit != nil is the
// test for one.
type Expr struct {
	Col  string
	Lit  any // string, int64, float64, or bool
	Fn   string
	Args []Expr
}

// UnmarshalYAML accepts a scalar literal, a {col: name} reference, or a
// single-key {fn: arg} or {fn: [args]} call.
func (e *Expr) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var v any
		if err := n.Decode(&v); err != nil {
			return err
		}
		switch x := v.(type) {
		case string, int64, float64, bool:
		case int:
			v = int64(x)
		default:
			return fmt.Errorf("line %d: unsupported literal %v", n.Line, v)
		}
		e.Lit = v
		return nil
	case yaml.MappingNode:
		if len(n.Content) != 2 {
			return fmt.Errorf("line %d: expression must have exactly one key", n.Line)
		}
		key, val := n.Content[0].Value, n.Content[1]
		if key == "col" {
			return val.Decode(&e.Col)
		}
		e.Fn = key
		if val.Kind == yaml.SequenceNode {
			return val.Decode(&e.Args)
		}
		var arg Expr
		if err := val.Decode(&arg); err != nil {
			return err
		}
		e.Args = []Expr{arg}
		return nil
	}
	return fmt.Errorf("line %d: invalid expression", n.Line)
}

// Parse checks a yaml document against the grammar and decodes it. Parse
// knows nothing about schemas; column and type checks happen in Compile.
func Parse(data []byte) (*Definition, error) {
	if err := validateGrammar(data); err != nil {
		return nil, fmt.Errorf("transform grammar: %w", err)
	}
	var d Definition
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parse transform: %w", err)
	}
	if d.Version != supportedVersion {
		return nil, fmt.Errorf("unsupported transform version %d (want %d)", d.Version, supportedVersion)
	}
	return &d, nil
}
