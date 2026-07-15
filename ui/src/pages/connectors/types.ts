import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export interface ConnectorsPageState {
  search: string;
  kindFilter: ConnectorKind;
}
