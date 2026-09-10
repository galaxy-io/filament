package server

import (
	"fmt"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/notifier"
)

func notifierFromInput(in *ingestionv1.NotifierInput) (notifier.Notifier, error) {
	if in == nil {
		return notifier.Notifier{}, fmt.Errorf("notifier is required")
	}
	var kind notifier.NotificationType
	switch in.GetNotificationType() {
	case ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK:
		kind = notifier.NotificationWebhook
	default:
		return notifier.Notifier{}, fmt.Errorf("unsupported notification type")
	}
	return notifier.NormalizeAndValidate(notifier.Notifier{
		Name: in.GetName(), NotificationType: kind, IsEnabled: in.GetIsEnabled(),
		Events: in.GetEvents(), Resources: in.GetResources(),
	}, events.Names())
}

func notifierToProto(n notifier.Notifier) *ingestionv1.Notifier {
	kind := ingestionv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	if n.NotificationType == notifier.NotificationWebhook {
		kind = ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK
	}
	return &ingestionv1.Notifier{
		Id: n.ID, TenantId: string(n.Tenant), PipelineId: n.PipelineID,
		Name: n.Name, NotificationType: kind, IsEnabled: n.IsEnabled,
		Events: n.Events, Resources: n.Resources, SecretRefs: cloneStrings(n.SecretRefs), Version: n.Version,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, DeletedAt: n.DeletedAt,
		CreatedByUserId: n.CreatedByUserID, UpdatedByUserId: n.UpdatedByUserID, DeletedByUserId: n.DeletedByUserID,
	}
}
