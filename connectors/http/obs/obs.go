// Package obs holds tiny constructors that guarantee non-nil observability
// dependencies (logger, reporter). Use these at every entry point so call
// sites never need `if x == nil` guards.
//
// Rationale: scattered nil checks lose instrumentation silently when a caller
// forgets to wire a reporter — events drop on the floor. Constructing through
// these helpers makes the absence of a real reporter a visible "Noop"
// (still working) rather than a dropped call (silent).
//
// # Worked example
//
// At the entry of any extract/configure/teardown method:
//
//	c.logger = obs.Logger(opts.Logger)       // never nil after this line
//	c.reporter = obs.Reporter(opts.Reporter) // never nil after this line
//	// safe to call c.logger.Info / c.reporter.Report unconditionally
//
// # Failure modes
//
// None — both helpers are infallible. nil input maps to safe defaults
// (slog.Default(), pipeline.NoopReporter{}).
package obs

import (
	"log/slog"

	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
)

// Logger returns l when non-nil, else slog.Default(). The returned logger is
// always safe to call.
func Logger(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

// Reporter returns r when non-nil, else pipeline.NoopReporter{}. The returned
// reporter is always safe to call.
func Reporter(r pipeline.Reporter) pipeline.Reporter {
	if r != nil {
		return r
	}
	return pipeline.NoopReporter{}
}
