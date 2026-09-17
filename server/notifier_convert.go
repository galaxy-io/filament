package server

import (
	"fmt"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

// notifierFromInput validates the generic settings and separates the
// submitted config from the caller's secret references.
func notifierFromInput(in *ingestionv1.NotifierInput, tenant string) (*ingestionv1.Notifier, map[string]any, map[string]string, error) {
	if in == nil {
		return nil, nil, nil, fmt.Errorf("notifier is required")
	}
	switch in.GetNotificationType() {
	case ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK:
	default:
		return nil, nil, nil, fmt.Errorf("unsupported notification type")
	}
	n, err := notifier.NormalizeAndValidate(&ingestionv1.Notifier{
		Name: in.GetName(), NotificationType: in.GetNotificationType(), IsEnabled: in.GetIsEnabled(),
		Events: in.GetEvents(), Resources: in.GetResources(),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	refs := cloneStrings(in.GetSecretRefs())
	if err := validateSecretRefTenant(refs, tenant); err != nil {
		return nil, nil, nil, err
	}
	return n, structMap(in.GetConfig()), refs, nil
}
