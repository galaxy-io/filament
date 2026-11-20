package connection

import (
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestValidateURI(t *testing.T) {
	ok := filament.NewConfig(map[string]any{"uri": "rabbitmq-stream://guest:guest@localhost:5552/%2f"})
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	tls := filament.NewConfig(map[string]any{"uri": "rabbitmq-stream+tls://guest:guest@broker:5551/prod"})
	if err := Validate(tls); err != nil {
		t.Fatal(err)
	}
	for _, uri := range []string{"", "amqp://localhost", "rabbitmq-stream://:5552/%2f", "rabbitmq-stream://localhost:5552/%2f#frag"} {
		err := Validate(filament.NewConfig(map[string]any{"uri": uri}))
		if err == nil {
			t.Fatalf("uri %q accepted", uri)
		}
	}
	if err := Validate(filament.NewConfig(nil)); err == nil || !strings.Contains(err.Error(), "uri is required") {
		t.Fatal(err)
	}
}
