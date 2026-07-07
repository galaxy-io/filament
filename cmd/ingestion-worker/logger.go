package main

import (
	"log/slog"

	ingestion "github.com/galaxy-io/filament"
)

// slogLogger adapts log/slog to ingestion.Logger so run facts and runner
// errors mirror to the worker's stdout alongside the event bus.
type slogLogger struct {
	l *slog.Logger
}

func (s slogLogger) Debug(msg string, kv ...ingestion.Field) { s.l.Debug(msg, args(kv)...) }
func (s slogLogger) Info(msg string, kv ...ingestion.Field)  { s.l.Info(msg, args(kv)...) }
func (s slogLogger) Warn(msg string, kv ...ingestion.Field)  { s.l.Warn(msg, args(kv)...) }

func (s slogLogger) Error(msg string, err error, kv ...ingestion.Field) {
	s.l.Error(msg, append(args(kv), "error", err)...)
}

func (s slogLogger) With(kv ...ingestion.Field) ingestion.Logger {
	return slogLogger{l: s.l.With(args(kv)...)}
}

func args(kv []ingestion.Field) []any {
	out := make([]any, 0, len(kv)*2)
	for _, f := range kv {
		out = append(out, f.Key, f.Value)
	}
	return out
}
