package streamcontrol

import (
	"errors"
	"fmt"
	"github.com/galaxy-io/filament"
)

// ValidateProfile is shared by graph validation, compilation and worker startup.
func ValidateProfile(store filament.DataStore, source filament.Source, sink filament.Sink) error {
	if _, ok := store.(Store); !ok {
		return fmt.Errorf("continuous execution requires PostgreSQL runtime persistence")
	}
	if source.Spec().Name != "nats" || sink.Spec().Name != "postgres" {
		return errors.New("continuous execution supports NATS JetStream to PostgreSQL append")
	}
	if _, ok := source.(filament.StreamSource); !ok {
		return errors.New("source has no native stream session")
	}
	if _, ok := sink.(filament.StreamingSink); !ok {
		return errors.New("sink has no native epoch lifecycle")
	}
	return nil
}
