// Package source consumes NATS JetStream messages as continuous resources.
//
// Planning fixes logical subject patterns (or explicit stream/consumer pairs).
// At stream open, resource resolution maps patterns to physical streams and
// verifies checkpoint domains before creating or binding durable consumers.
// Source owns the connection; each session owns a consumerBinding and its pull
// subscription. multiSession multiplexes those sessions and shares row writers
// when a logical resource spans physical streams.
//
// Sessions emit sequence positions and acknowledge messages only after the
// coordinator certifies coverage. Shared lifecycle bookkeeping lives in
// internal/stream; consumer validation and JetStream operations stay here.
package source
