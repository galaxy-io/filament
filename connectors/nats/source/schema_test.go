package source

import (
	"testing"

	"github.com/nats-io/nats.go"
)

func TestHeaderPresenceAndRepeatedValues(t *testing.T) {
	if messageHeaders(&nats.Msg{}) != nil {
		t.Fatal("absent headers became present")
	}
	if messageHeaders(&nats.Msg{Header: nats.Header{}}) == nil {
		t.Fatal("empty headers became absent")
	}
	headers := messageHeaders(&nats.Msg{Header: nats.Header{"Z": []string{"first", "second"}, "A": []string{""}}})
	if len(headers) != 3 || headers[0].Key != "A" || headers[0].Value == nil || string(headers[1].Value) != "first" || string(headers[2].Value) != "second" {
		t.Fatalf("header fidelity: %+v", headers)
	}
}
