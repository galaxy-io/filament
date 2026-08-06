# Profiling helpers

`profiling` provides two standard-library-only ways to inspect a Go process.

For live profiling, mount the pprof endpoints on an existing internal HTTP
listener:

```go
mux := http.NewServeMux()
profiling.Mount(mux)
```

Or run a dedicated diagnostics listener until a context is canceled:

```go
go func() {
	if err := profiling.Serve(ctx, "127.0.0.1:6060"); err != nil {
		log.Printf("profiling server: %v", err)
	}
}()
```

Then collect a 30-second CPU profile or inspect the current heap:

```sh
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

For a bounded file-based session, the default captures CPU samples and a heap
snapshot:

```go
session, err := profiling.Start(profiling.DefaultConfig("./profiles"))
if err != nil {
	return err
}
defer func() {
	if err := session.Stop(); err != nil {
		log.Printf("stop profiling: %v", err)
	}
}()
```

Select additional runtime profiles through `Config.Profiles`:

```go
cfg := profiling.DefaultConfig("./profiles")
cfg.Profiles = append(cfg.Profiles,
	profiling.AllocsProfile,
	profiling.GoroutineProfile,
	profiling.BlockProfile,
	profiling.MutexProfile,
	profiling.TraceProfile,
)
session, err := profiling.Start(cfg)
```

## Open captured files in a browser

Use the pprof web UI for CPU, heap, allocations, goroutine, block, mutex, and
thread-creation profiles:

```sh
go tool pprof -http=:0 ./profiles/20260806T120000.000000000Z-cpu.pprof
go tool pprof -http=:0 ./profiles/20260806T120000.000000000Z-heap.pprof
```

Replace the example path with the file written by your session. `:0` selects an
available local port; pprof prints the address and normally opens it in your
default browser. Keep the command running while using the web interface, and
press `Ctrl-C` when finished.

If symbol or source information is missing, provide the exact executable that
created the capture as well:

```sh
go tool pprof -http=:0 ./bin/filament/worker ./profiles/my-run-cpu.pprof
```

Runtime traces use Go's separate trace web viewer:

```sh
go tool trace ./profiles/20260806T120000.000000000Z-trace.out
```

This also starts a local web server and normally opens the trace interface in
your browser. Press `Ctrl-C` when finished.

Existing files are never overwritten; use a new prefix or remove an old capture
before reusing an explicit `Config.Prefix`.
Profiling output and live endpoints can contain sensitive process details;
store files securely and expose the HTTP listener only on a trusted network.
The standard `net/http/pprof` package registers its routes on
`http.DefaultServeMux` when imported, so do not serve the default mux on a
public listener.
