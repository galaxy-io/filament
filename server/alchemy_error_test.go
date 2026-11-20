package server

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
)

func TestAlchemyErrorKeepsCallerCodes(t *testing.T) {
	canceled := connect.NewError(connect.CodeCanceled, context.Canceled)
	if got := connect.CodeOf(alchemyError("op", canceled)); got != connect.CodeCanceled {
		t.Fatalf("canceled became %v", got)
	}
	if got := connect.CodeOf(alchemyError("op", errors.New("dial tcp: refused"))); got != connect.CodeUnavailable {
		t.Fatalf("transport failure became %v", got)
	}
}
