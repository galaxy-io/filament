import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export interface ConnectionsPageState {
  search: string;
  kindFilter: ConnectorKind;
}
