// Package uiconfig reads what the server tells the web UI at runtime.
package uiconfig

import (
	"os"

	"github.com/galaxy-io/filament/ui"
)

// FromEnv reads GALAXY_<PRODUCT>_URL, where each other Galaxy product's UI is
// served. Unset products get no links.
func FromEnv() (ui.Config, error) {
	productURLs, err := ui.ProductURLsFromEnv(os.Getenv)
	if err != nil {
		return ui.Config{}, err
	}
	return ui.Config{ProductURLs: productURLs}, nil
}
