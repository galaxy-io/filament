// Package uiconfig reads what the server tells the web UI at runtime.
package uiconfig

import (
	"fmt"
	"os"

	"github.com/galaxy-io/filament/ui"
)

// FromEnv reads ALCHEMY_URL, where Alchemy's UI is served. Unset means the UI
// offers no Alchemy links.
func FromEnv() (ui.Config, error) {
	alchemyURL, err := ui.ParseAlchemyURL(os.Getenv("ALCHEMY_URL"))
	if err != nil {
		return ui.Config{}, fmt.Errorf("ALCHEMY_URL: %w", err)
	}
	return ui.Config{AlchemyURL: alchemyURL}, nil
}
