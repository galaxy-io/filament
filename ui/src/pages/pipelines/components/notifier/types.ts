import type { Notifier } from "@/gen/ingestion/v1/notifiers_pb";

export interface PipelineNotifierState {
  name: Notifier["name"];
  notificationType: Notifier["notificationType"];
  isEnabled: Notifier["isEnabled"];
  events: Notifier["events"];
  url: string;
  headers: string;
  secretRefs: Notifier["secretRefs"];
}

export interface PipelineNotifier extends PipelineNotifierState {
  id: Notifier["id"];
}
