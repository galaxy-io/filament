package source

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

func managedDomain(identity, resource string, info *nats.StreamInfo) filament.DomainKey {
	return filament.DomainKey{Domain: fmt.Sprintf("%d:%s%s", len(resource), resource, info.Config.Name), Incarnation: fmt.Sprintf("%d:%s%s", len(identity), identity, info.Created.UTC().Format(time.RFC3339Nano))}
}

// consumerDomain preserves the checkpoint namespace of explicit stream consumers.
func consumerDomain(identity, physical string, info *nats.StreamInfo) filament.DomainKey {
	// Length-prefix components to avoid identity collisions with separators.
	incarnation := fmt.Sprintf("%d:%s%d:%s%s", len(identity), identity, len(physical), physical, info.Created.UTC().Format(time.RFC3339Nano))
	return filament.DomainKey{Domain: physical, Incarnation: incarnation}
}
