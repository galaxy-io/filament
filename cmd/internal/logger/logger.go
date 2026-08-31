// Package logger adapts log/slog to filament.Logger so every command binary
// emits one structured format. Modules take the result through module.Deps.Log;
// without it their log calls are guarded no-ops.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/galaxy-io/filament"
)

// LevelTrace is finer grained than slog's built-in Debug level.
const LevelTrace = slog.LevelDebug - 4

// New returns a Logger writing JSON to stdout at LOG_LEVEL (INFO by default).
// Valid levels are INFO, DEBUG, and TRACE. It also installs the underlying
// slog.Logger as the process default, so library logs (e.g. iceberg-go) come
// out in the same format.
func New() (filament.Logger, error) {
	level, err := levelFromEnv()
	if err != nil {
		return nil, err
	}
	return newLogger(os.Stdout, level), nil
}

func newLogger(w io.Writer, level slog.Level) filament.Logger {
	l := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.LevelKey && attr.Value.Any() == LevelTrace {
				attr.Value = slog.StringValue("TRACE")
			}
			return attr
		},
	}))
	slog.SetDefault(l)
	return adapter{l: l}
}

func levelFromEnv() (slog.Level, error) {
	switch level := strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_LEVEL"))); level {
	case "", "INFO":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "TRACE":
		return LevelTrace, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q is invalid (INFO, DEBUG, TRACE)", level)
	}
}

type adapter struct {
	l *slog.Logger
}

var _ filament.Logger = adapter{}

func (a adapter) Trace(msg string, kv ...filament.Field) {
	a.l.Log(context.Background(), LevelTrace, msg, args(kv)...)
}

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
