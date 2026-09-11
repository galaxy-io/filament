package notifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/galaxy-io/filament"
)

// Notification is the in-memory input to a sender. Config contains resolved secrets.
type Notification struct {
	NotifierID            string
	NotifierVersion       int64
	NotificationType      NotificationType
	Tenant                filament.TenantID
	Run                   filament.RunID
	Resource              string
	PipelineID            string
	PipelineVersionID     string
	DeliveryID            string
	AttemptID             string
	TriggerType           string
	TriggerSubject        string
	TriggerStreamSequence uint64
	Event                 json.RawMessage
	Config                json.RawMessage `json:"-"`
}

// Outcome describes what the sender observed, not downstream processing.
type Outcome string

// Supported delivery outcomes.
const (
	OutcomeAccepted Outcome = "accepted"
	OutcomeFailed   Outcome = "failed"
	OutcomeUnknown  Outcome = "unknown"
)

// ErrorCode is a safe classification for events and metrics. Never use raw errors.
type ErrorCode string

// Safe error codes for notification reports.
const (
	ErrorNone                 ErrorCode = ""
	ErrorTimeout              ErrorCode = "timeout"
	ErrorTransport            ErrorCode = "transport_error"
	ErrorHTTPRejected         ErrorCode = "http_rejected"
	ErrorRateLimited          ErrorCode = "rate_limited"
	ErrorSecretUnavailable    ErrorCode = "secret_unavailable"
	ErrorInvalidConfiguration ErrorCode = "invalid_configuration"
	ErrorInternal             ErrorCode = "internal_error"
)

// DeliveryResult records one observed operation. RequestAttempted does not prove receipt.
type DeliveryResult struct {
	Outcome          Outcome
	Retryable        bool
	RequestAttempted bool
	StatusCode       int
	Duration         time.Duration
	ErrorCode        ErrorCode
}

// Sender delivers one notification and reports its outcome.
type Sender interface {
	Send(context.Context, Notification) DeliveryResult
}

// DeliveryID is stable for a notifier and source message, including redelivery.
// Stream recreation that reuses sequence numbers is outside this guarantee.
func DeliveryID(notifierID, subject string, sequence uint64) (string, error) {
	if notifierID == "" || subject == "" || sequence == 0 || !utf8.ValidString(notifierID) || !utf8.ValidString(subject) {
		return "", fmt.Errorf("notifier: delivery identity requires an ID, subject, and non-zero stream sequence")
	}
	raw, err := json.Marshal([]string{"notifier.delivery.v1", notifierID, subject, strconv.FormatUint(sequence, 10)})
	if err != nil {
		return "", fmt.Errorf("notifier: encode delivery identity: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
