// Package version reports the build version stamped into the binaries.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Set at build time with -ldflags "-X github.com/galaxy-io/filament/cmd/internal/version.Version=...".
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
