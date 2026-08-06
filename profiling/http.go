package profiling

import (
	"context"
	"errors"
	"net/http"
	httppprof "net/http/pprof"
	"time"
)

const shutdownTimeout = 5 * time.Second

// Mux is the part of http.ServeMux needed by Mount.
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// Mount registers the standard Go pprof endpoints below /debug/pprof/.
//
// Profiling data can expose implementation and request details. Mount these
// handlers only on an internal or otherwise access-controlled listener.
func Mount(mux Mux) {
	// Importing net/http/pprof registers these routes on DefaultServeMux.
	// Avoid the duplicate-pattern panic when callers pass it explicitly.
	if serveMux, ok := mux.(*http.ServeMux); ok && serveMux == http.DefaultServeMux {
		return
	}
	mux.Handle("GET /debug/pprof/", http.HandlerFunc(httppprof.Index))
	mux.Handle("GET /debug/pprof/cmdline", http.HandlerFunc(httppprof.Cmdline))
	mux.Handle("GET /debug/pprof/profile", http.HandlerFunc(httppprof.Profile))
	mux.Handle("GET /debug/pprof/symbol", http.HandlerFunc(httppprof.Symbol))
	mux.Handle("GET /debug/pprof/trace", http.HandlerFunc(httppprof.Trace))

	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("GET /debug/pprof/"+name, httppprof.Handler(name))
	}
}

// Handler returns an isolated handler containing only the pprof endpoints.
// It is useful for a dedicated diagnostics listener.
func Handler() http.Handler {
	mux := http.NewServeMux()
	Mount(mux)
	return mux
}

// Serve exposes the pprof endpoints at addr until ctx is canceled. It shuts
// the server down gracefully and treats http.ErrServerClosed as success.
// Prefer a loopback or private address, for example "127.0.0.1:6060".
func Serve(ctx context.Context, addr string) error {
	if err := ctx.Err(); err != nil {
		return nil
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return normalizeServerError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		var closeErr error
		if shutdownErr != nil {
			closeErr = srv.Close()
		}
		serveErr := normalizeServerError(<-errCh)
		return errors.Join(shutdownErr, closeErr, serveErr)
	}
}

func normalizeServerError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
