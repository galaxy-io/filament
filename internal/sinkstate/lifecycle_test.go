package sinkstate

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestFailResourceBlocksOnlyThatResource(t *testing.T) {
	var s Lifecycle
	s.Open()
	if err := s.MarkBegun(); err != nil {
		t.Fatal(err)
	}
	err := s.FailResource("a", errors.New("constraint violated"))
	var re *filament.ResourceError
	if !errors.As(err, &re) || re.Resource != "a" {
		t.Fatalf("FailResource = %v, want a ResourceError for a", err)
	}
	if err := s.CheckApplyResource("a"); !errors.As(err, &re) || !errors.Is(err, ErrState) {
		t.Fatalf("CheckApplyResource(a) = %v, want a blocked ResourceError", err)
	}
	if err := s.CheckApplyResource("b"); err != nil {
		t.Fatalf("CheckApplyResource(b) = %v, want nil", err)
	}
	if err := s.CheckApply(); err != nil {
		t.Fatalf("CheckApply = %v: a resource failure must not poison the session", err)
	}
	if err := s.MarkCommitted(); err != nil {
		t.Fatalf("MarkCommitted = %v", err)
	}
	if err := s.MarkBegun(); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckApplyResource("a"); err != nil {
		t.Fatalf("next boundary CheckApplyResource(a) = %v, want nil", err)
	}
	s.Fail(errors.New("connection lost"))
	if err := s.CheckApplyResource("b"); err == nil {
		t.Fatal("CheckApplyResource after a session failure = nil")
	}
}
