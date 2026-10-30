// Package version reports the build version stamped into the binaries.
package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
)

// Set at build time with -ldflags "-X github.com/galaxy-io/filament/version.Version=...".
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Fill in from Go's embedded build info when nothing was stamped.
func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" && !strings.HasPrefix(info.Main.Version, "v0.0.0-") {
		Version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if Commit == "unknown" && len(setting.Value) >= 7 {
				Commit = setting.Value[:7]
			}
		case "vcs.time":
			if Date == "unknown" {
				Date = setting.Value
			}
		case "vcs.modified":
			if setting.Value == "true" && !strings.Contains(Version, "dirty") {
				Version += "-dirty"
			}
		}
	}
}

// String renders the version with its commit and build date.
func String() string {
	return fmt.Sprintf("%s (%s, %s)", Version, Commit, Date)
}

// Info is the JSON body served by Handler.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Handler serves the stamped version as JSON on GET /version.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Info{Version: Version, Commit: Commit, Date: Date})
	})
}
