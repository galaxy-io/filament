// Package logger adapts log/slog to filament.Logger so every command binary
// emits one structured format. Modules take the result through module.Deps.Log;
// without it their log calls are guarded no-ops.
package logger

import (
	"log/slog"
	"os"

	"github.com/galaxy-io/filament"
)

// New returns a Logger writing JSON to stdout. It also installs the underlying
// slog.Logger as the process default, so library logs (e.g. iceberg-go) come
// out in the same format.
func New() filament.Logger {
	l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(l)
	return adapter{l: l}
}

type adapter struct {
	l *slog.Logger
}

var _ filament.Logger = adapter{}

func (a adapter) Debug(msg string, kv ...filament.Field) { a.l.Debug(msg, args(kv)...) }
func (a adapter) Info(msg string, kv ...filament.Field)  { a.l.Info(msg, args(kv)...) }
func (a adapter) Warn(msg string, kv ...filament.Field)  { a.l.Warn(msg, args(kv)...) }

func (a adapter) Error(msg string, err error, kv ...filament.Field) {
	a.l.Error(msg, append(args(kv), "error", err)...)
}

func (a adapter) With(kv ...filament.Field) filament.Logger {
	return adapter{l: a.l.With(args(kv)...)}
}

func args(kv []filament.Field) []any {
	out := make([]any, 0, len(kv)*2)
	for _, f := range kv {
		out = append(out, f.Key, f.Value)
	}
	return out
}
