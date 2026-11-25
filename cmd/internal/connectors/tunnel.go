package connectors

import (
	"os"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// sourcesWithReverseTunnel hands SSH_REVERSE_TUNNEL_HOST, where this
// deployment's reverse SSH tunnels listen, to every source that can dial one.
// Unset leaves the registry as it is.
func sourcesWithReverseTunnel(sources *registry.Sources) *registry.Sources {
	host := os.Getenv("SSH_REVERSE_TUNNEL_HOST")
	if host == "" {
		return sources
	}
	return sources.Configured(func(source filament.Source) { useReverseTunnelHost(source, host) })
}

// sinksWithReverseTunnel does the same for sinks.
func sinksWithReverseTunnel(sinks *registry.Sinks) *registry.Sinks {
	host := os.Getenv("SSH_REVERSE_TUNNEL_HOST")
	if host == "" {
		return sinks
	}
	return sinks.Configured(func(sink filament.Sink) { useReverseTunnelHost(sink, host) })
}

func useReverseTunnelHost(connector any, host string) {
	if user, ok := connector.(filament.ReverseTunnelUser); ok {
		user.UseReverseTunnelHost(host)
	}
}
