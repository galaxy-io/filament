package rhoapi

import (
	_ "embed"
	"github.com/galaxy-io/filament"
	httpapi "github.com/galaxy-io/filament/connectors/http"
)

//go:embed manifest.yaml
var manifestData []byte

func NewRho() *httpapi.Source {
	return httpapi.NewManifestWithMetadata(
		"rho",
		"Rho",
		"Business banking platform offering checking, savings, corporate cards, and treasury services.",
		"https://cdn.getgalaxy.io/sources/source-icon-rho-dark.svg",
		"https://cdn.getgalaxy.io/sources/source-icon-rho-light.svg",
		manifestData,
		filament.ConfigSchema{},
	)
}
