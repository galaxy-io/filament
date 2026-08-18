// Package obs holds constructors for non-nil observability dependencies.
package obs

import "log/slog"

// Logger returns l when non-nil, else slog.Default(). The returned logger is
// always safe to call.
func Logger(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}
