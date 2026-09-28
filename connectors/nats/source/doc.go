// Package source consumes NATS JetStream messages as continuous resources.
//
// Planning fixes logical subject patterns (or explicit stream/consumer pairs).
// At stream open, resource resolution maps patterns to physical streams and
// verifies checkpoint domains before creating or binding durable consumers.
// Source owns the connection; each session owns a consumerBinding and its pull
// subscription. multiSession multiplexes those sessions and shares row writers
// when a logical resource spans physical streams.
//
// JSON object payloads become top-level source columns, with the same projection
// rules as Kafka. The first message fixes each logical resource's schema;
// missing values are null and incompatible changes fail before certification.
// Non-object payloads retain raw bytes in a source-owned payload column.
// Filament event metadata is separate and never contains _filament_payload.
//
// Managed filtered consumers read retained matching messages. Stream sequence
// gaps are normal for filters, and JetStream's stream-wide deletion metadata
// cannot distinguish unrelated deletions from matching input lost to retention.
// Retention must cover the maximum processing/recovery lag; loss beyond that
// window is not detectable. Explicit unfiltered consumers retain strict gap checks.
// Managed consumers allow up to 1000 pending messages per certified epoch;
// legacy managed consumers are upgraded in place. Explicit consumers keep their
// required single-message credit. All pending messages receive heartbeats and
// remain unacknowledged until the coordinator certifies the batch.
//
// Sessions emit sequence positions and acknowledge messages only after the
// coordinator certifies coverage. Shared lifecycle bookkeeping lives in
// internal/stream; consumer validation and JetStream operations stay here.
package source
