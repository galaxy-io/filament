package inline_test

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/cmd/internal/inline"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"

	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

func TestRunWritesEveryFactOfTheRun(t *testing.T) {
	spec := `{"Tenant":"local","Run":"cli-test","Source":{"Connector":"sample","Config":{"rows":2}},"Sink":{"Connector":"stdout","Config":{}},"Resources":["users"]}`
	var facts bytes.Buffer
	if err := inline.Run(t.Context(), registry.DefaultSources, registry.DefaultSinks, strings.NewReader(spec+"\n"), &facts); err != nil {
		t.Fatal(err)
	}
	names := []string{}
	lines := bufio.NewScanner(&facts)
	for lines.Scan() {
		fact, err := events.Unmarshal(lines.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, fact.Name)
	}
	if len(names) == 0 || names[0] != events.RunStarted.Name() || names[len(names)-1] != events.RunCompleted.Name() {
		t.Fatalf("facts = %v, want run.started through run.completed", names)
	}
}

func TestRunRejectsMissingSpec(t *testing.T) {
	if err := inline.Run(t.Context(), registry.DefaultSources, registry.DefaultSinks, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Fatal("run without a spec succeeded")
	}
}
