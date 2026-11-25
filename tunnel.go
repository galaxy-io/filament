package filament

// ReverseTunnelUser is implemented by a source or sink that can dial a reverse
// SSH tunnel. The binary hands it the host where this deployment's reverse
// tunnels listen before the connector is asked for its spec or opened. An
// empty host leaves the reverse method out.
type ReverseTunnelUser interface {
	UseReverseTunnelHost(host string)
}
