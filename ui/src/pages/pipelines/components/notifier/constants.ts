import type { PinnedOptions } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { NotificationType, NotifierEvent } from "@/gen/ingestion/v1/notifiers_pb";

import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";

export const PIPELINE_NOTIFIER_HEADERS_SECRET_REF_KEY = "headers";

export const PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED = 64;

export const PIPELINE_NOTIFIER_HEADERS_PLACEHOLDER_TEXT = `{
  "Authorization": "Bearer <token>"
}`;
export const PIPELINE_NOTIFIER_HEADERS_KEEP_PLACEHOLDER_TEXT = `{
  "Leave blank to keep current value": ""
}`;

export const PIPELINE_NOTIFIER_TYPE_TO_LABEL_MAP: Record<NotificationType, string> = {
  [NotificationType.UNSPECIFIED]: "Unknown",
  [NotificationType.WEBHOOK]: "Webhook",
};

export const PIPELINE_NOTIFIER_TYPES = Object.values(NotificationType).filter(
  (type): type is NotificationType =>
    typeof type === "number" && type !== NotificationType.UNSPECIFIED,
);

export const PIPELINE_NOTIFIER_TYPE_OPTIONS: SelectInputOption[] = PIPELINE_NOTIFIER_TYPES.map(
  (type) => ({
    id: String(type),
    label: PIPELINE_NOTIFIER_TYPE_TO_LABEL_MAP[type],
    value: type,
  }),
);

export const PIPELINE_NOTIFIER_EVENT_TO_LABEL_MAP: Record<NotifierEvent, string> = {
  [NotifierEvent.UNSPECIFIED]: "Unknown",
  [NotifierEvent.RUN_STARTED]: "run.started",
  [NotifierEvent.RUN_COMPLETED]: "run.completed",
  [NotifierEvent.RUN_FAILED]: "run.failed",
  [NotifierEvent.RUN_PARTIAL]: "run.partial",
  [NotifierEvent.RUN_CANCELED]: "run.canceled",
  [NotifierEvent.RUN_PAUSED]: "run.paused",
};

export const PIPELINE_NOTIFIER_EVENTS = Object.values(NotifierEvent).filter(
  (event): event is NotifierEvent =>
    typeof event === "number" && event !== NotifierEvent.UNSPECIFIED,
);

export const PIPELINE_NOTIFIER_EVENT_OPTIONS: SelectInputOption[] = PIPELINE_NOTIFIER_EVENTS.map(
  (event) => ({
    id: String(event),
    label: PIPELINE_NOTIFIER_EVENT_TO_LABEL_MAP[event],
    value: event,
  }),
);

export const PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION: PinnedOptions = {
  id: "all-events",
  label: "All events",
  optionIds: PIPELINE_NOTIFIER_EVENT_OPTIONS.map((option) => option.id),
};

export const PIPELINE_NOTIFIER_DEFAULT_STATE: PipelineNotifierState = {
  name: "",
  notificationType: NotificationType.WEBHOOK,
  isEnabled: true,
  events: [NotifierEvent.RUN_FAILED],
  url: "",
  headers: "",
  secretRefs: {},
};
