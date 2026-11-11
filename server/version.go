package server

import (
	"context"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/version"
)

// GetVersion returns the server's build version, commit and date.
func (*Server) GetVersion(context.Context, *connect.Request[ingestionv1.GetVersionRequest]) (*connect.Response[ingestionv1.GetVersionResponse], error) {
	return connect.NewResponse(&ingestionv1.GetVersionResponse{
		Version: version.Version, Commit: version.Commit, Date: version.Date,
	}), nil
}
