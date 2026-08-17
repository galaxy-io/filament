// Package dispatch defines behavior shared by all run dispatch backends.
package dispatch

// Durable is the transport-neutral consumer name for run dispatch. Keeping it
// stable across in-process and Kubernetes backends preserves one logical
// consumer when the dispatch implementation changes.
const Durable = "dispatch"
