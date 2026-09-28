package source

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
)

func managedDomain(identity, resource string, info *jetstream.StreamInfo) filament.DomainKey {
	return filament.DomainKey{Domain: fmt.Sprintf("%d:%s%s", len(resource), resource, info.Config.Name), Incarnation: fmt.Sprintf("%d:%s%s", len(identity), identity, info.Created.UTC().Format(time.RFC3339Nano))}
}

// consumerDomain preserves the checkpoint namespace of explicit stream consumers.
func consumerDomain(identity, physical string, info *jetstream.StreamInfo) filament.DomainKey {
	// Length-prefix components to avoid identity collisions with separators.
	incarnation := fmt.Sprintf("%d:%s%d:%s%s", len(identity), identity, len(physical), physical, info.Created.UTC().Format(time.RFC3339Nano))
	return filament.DomainKey{Domain: physical, Incarnation: incarnation}
}

// sourceIdentity uses the stable Filament connection ID; neither endpoints nor
// worker attempts are identity.
func sourceIdentity(connectionID string) (string, error) {
	if connectionID == "" {
		return "", fmt.Errorf("nats: source connection ID is required for stream identity")
	}
	return connectionID, nil
}
