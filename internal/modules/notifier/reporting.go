package notifier

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
	notification "github.com/galaxy-io/filament/internal/notifier"
)

func (m *Module) report(ctx context.Context, a attempt) {
	// Finish an observed attempt even when shutdown cancels the handler.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reportTimeout)
	defer cancel()
	n, r := a.notification, a.result
	kind, err := n.NotificationType.Label()
	if err != nil {
		kind = "unknown"
	}
	m.observe(kind, r)
	fields := []filament.Field{
		{Key: "notifier_id", Value: n.NotifierID},
		{Key: "tenant_id", Value: n.Tenant},
		{Key: "pipeline_id", Value: n.PipelineID},
		{Key: "run_id", Value: n.Run},
		{Key: "delivery_id", Value: n.DeliveryID},
		{Key: "attempt_id", Value: n.AttemptID},
		{Key: "outcome", Value: r.Outcome},
		{Key: "error_code", Value: r.ErrorCode},
	}
	if m.log != nil {
		m.log.Info("notification operation completed", fields...)
	}
	if err == nil {
		err = events.Emit(ctx, m.bus, events.NotifierAttempted, events.Envelope{
			Tenant: n.Tenant, Run: n.Run, Resource: n.Resource, At: a.completedAt,
		}, events.NotifierAttemptedEvent{
			NotifierID: n.NotifierID, NotifierVersion: n.NotifierVersion, NotificationType: n.NotificationType,
			PipelineID: n.PipelineID, PipelineVersionID: n.PipelineVersionID,
			DeliveryID: n.DeliveryID, AttemptID: n.AttemptID,
			TriggerType: n.TriggerType, TriggerSubject: n.TriggerSubject, TriggerStreamSequence: n.TriggerStreamSequence,
			Outcome: r.Outcome, RequestAttempted: r.RequestAttempted, Retryable: r.Retryable,
			StatusCode: r.StatusCode, DurationMs: r.Duration.Milliseconds(), ErrorCode: r.ErrorCode,
		})
	}
	if err != nil {
		if m.log != nil {
			m.log.Warn("notification attempt report failed", fields...)
		}
		if m.mx != nil {
			m.mx.Counter("filament_notifier_report_failures_total", filament.Label{Key: "notification_type", Value: kind}).Inc()
		}
	}
}

func (m *Module) observe(kind string, r notification.DeliveryResult) {
	if m.mx == nil {
		return
	}
	labels := []filament.Label{
		{Key: "notification_type", Value: kind},
		{Key: "outcome", Value: string(r.Outcome)},
	}
	m.mx.Counter("filament_notifier_operations_total", labels...).Inc()
	m.mx.Histogram("filament_notifier_operation_duration_seconds", labels...).Observe(r.Duration.Seconds())
	if r.RequestAttempted {
		m.mx.Counter("filament_notifier_requests_total", labels...).Inc()
	}
	if r.ErrorCode == notification.ErrorSecretUnavailable || r.ErrorCode == notification.ErrorInvalidConfiguration {
		m.mx.Counter("filament_notifier_configuration_failures_total",
			filament.Label{Key: "notification_type", Value: kind},
			filament.Label{Key: "error_code", Value: string(r.ErrorCode)},
		).Inc()
	}
}
