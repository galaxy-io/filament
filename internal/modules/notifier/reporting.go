package notifier

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

func (m *Module) report(ctx context.Context, a attempt) {
	// Finish an observed attempt even when shutdown cancels the handler.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reportTimeout)
	defer cancel()
	n, r := a.notification, a.result
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
	_, err := n.NotificationType.Label()
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
	if err != nil && m.log != nil {
		m.log.Warn("notification attempt report failed", fields...)
	}
}
