package sink

import (
	"context"
	"errors"
	"testing"
)

func TestLifecycleOwnsRepeatedWritesAndFailure(t *testing.T) {
	s := New()
	s.lifecycle.Open()
	ctx := context.Background()
	for range 2 {
		if err := s.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("publish failed")
	s.fail(failure)
	if err := s.Commit(ctx); !errors.Is(err, failure) {
		t.Fatalf("lost failure: %v", err)
	}
	if err := s.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Begin(ctx); !errors.Is(err, failure) {
		t.Fatalf("reused failed session: %v", err)
	}
}

func TestCloseDistinguishesLocalFailureFromUnknownPublication(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		s := New()
		s.lifecycle.Open()
		failure := errors.New("write rejected")
		s.fail(failure)
		if uncertain {
			s.uncertain = failure
		}
		err := s.Close(context.Background())
		if uncertain && !errors.Is(err, failure) {
			t.Fatalf("unknown publication reported clean: %v", err)
		}
		if !uncertain && err != nil {
			t.Fatalf("local failure blocked clean shutdown: %v", err)
		}
	}
}
