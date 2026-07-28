# Plan 01 — Bundle size & delivery

**Goal:** first load goes from ~2 MB down to ~250–300 kB over the wire. Repeat loads are nearly free because assets cache forever.

**Server changes:** none to protos. Only the static file handler in `handler.go`.

**Today:**
- Main JS chunk: 2,002 kB raw / 686 kB gzip. No vendor splitting in `vite.config.ts`.
- `handler.go:26-43` serves files with plain `http.FileServer`: no compression, no cache headers. Users download the full 2 MB.
- `src/routes/__root.tsx:16-17` imports `CreateConnectionModal` and `ConnectionDrawer` at the top of the root route. That pulls the whole create-connection flow into the first load, even though it only shows when a modal opens.

## Step 1 — Compress and cache in `handler.go`

Use `gzhttp` from `github.com/klauspost/compress` (`go get` if not already in go.sum):

```go
import "github.com/klauspost/compress/gzhttp"

func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := dist.Open(path); err == nil {
				_ = f.Close()
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		r2.URL.RawPath = ""
		fileServer.ServeHTTP(w, r2)
	})
	return gzhttp.GzipHandler(inner)
}
```

Why this works:
- Vite puts a content hash in every filename under `assets/`. A file never changes at its URL, so the browser can cache it forever (`immutable`).
- `index.html` gets `no-cache` so a new deploy is picked up right away.
- `gzhttp` handles the `Accept-Encoding` negotiation and skips files that are already compressed (fonts, images).

## Step 2 — Split vendor code in `vite.config.ts`

Add to `build.rollupOptions`:

```ts
output: {
  manualChunks: (id) => {
    if (!id.includes("node_modules")) return;
    if (id.includes("@galaxy-io/dls")) return "dls";
    if (id.includes("@xyflow")) return "xyflow";
    if (
      id.includes("react-dom") ||
      id.includes("/react/") ||
      id.includes("@tanstack") ||
      id.includes("@connectrpc") ||
      id.includes("@bufbuild")
    ) {
      return "vendor";
    }
  },
},
```

Why: `vendor` and `dls` change rarely; app code changes every deploy. Split apart, a deploy only invalidates the small app chunk — everything else comes from browser cache.

## Step 3 — Load overlay flows on demand

In `src/routes/__root.tsx`, swap the static imports for lazy ones:

```tsx
import { lazy, Suspense } from "react";

const CreateConnectionModal = lazy(
  () => import("@/pages/connectors/components/create/CreateConnectionModal"),
);
const ConnectionDrawer = lazy(
  () => import("@/pages/connectors/components/drawer/ConnectionDrawer"),
);
```

Render the content only when the overlay is open, wrapped in `Suspense`:

```tsx
<Drawer open={!!connection} onClose={handleCloseDrawer} width={CONNECTOR_DRAWER_WIDTH}>
  {connection && (
    <Suspense fallback={null}>
      <ConnectionDrawer connection={connection} onClose={handleCloseDrawer} />
    </Suspense>
  )}
</Drawer>

<Modal open={flow === Flow.CREATE_CONNECTION} onClose={handleCloseFlow}>
  {flow === Flow.CREATE_CONNECTION && (
    <Suspense fallback={null}>
      <CreateConnectionModal onClose={handleCloseFlow} />
    </Suspense>
  )}
</Modal>
```

The `flow === ...` guard matters: without it the modal chunk loads even while closed.

## Verify

1. `pnpm build` — main chunk should drop well under the 500 kB warning. Note before/after sizes.
2. `just binaries`, run the server, then:
   `curl -sH 'Accept-Encoding: gzip' -D- http://localhost:<port>/assets/<hash>.js -o /dev/null`
   Expect `Content-Encoding: gzip` and `Cache-Control: ... immutable`.
3. In the browser, load `/pipelines` and watch the Network tab: the create-connection chunk should only appear when the modal opens.
4. Click through: canvas, history, settings, create connection, connection drawer.

## Done when

- First load is ≤ ~350 kB compressed.
- `assets/*` cache forever; `index.html` doesn't.
- The modal/drawer code is not in the entry chunk.
