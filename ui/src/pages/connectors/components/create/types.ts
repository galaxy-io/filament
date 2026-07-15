import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionModalStep {
  SELECT = "SELECT",
  CONFIGURE = "CONFIGURE",
}

export interface CreateConnectionModalProps {
  onClose: () => void;
}

export interface CreateConnectionSelectorProps {
  onClose: () => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}
