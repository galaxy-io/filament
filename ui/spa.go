package ui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	spaIndexFile          = "index.html"
	spaAssetsDir          = "assets/"
	spaAssetsCacheControl = "public, max-age=31536000, immutable"
	spaIndexCacheControl  = "no-cache"
)

// spaHandler serves the files in build and falls back to index.html for
// client routes, so deep links load the app. Hashed files under assets/ are
// cached forever, a missing one is a 404, and index.html is always revalidated.
func spaHandler(build fs.FS) http.Handler {
	fileServer := http.FileServerFS(build)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		isAsset := strings.HasPrefix(name, spaAssetsDir)
		if name != "" && name != spaIndexFile && isSPAFile(build, name) {
			if isAsset {
				w.Header().Set("Cache-Control", spaAssetsCacheControl)
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if isAsset && path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", spaIndexCacheControl)
		http.ServeFileFS(w, r, build, spaIndexFile)
	})
}

func isSPAFile(build fs.FS, name string) bool {
	info, err := fs.Stat(build, name)
	return err == nil && !info.IsDir()
}
