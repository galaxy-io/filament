import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionStep {
  SELECT = "select",
  CONFIGURE = "configure",
}

export interface CreateConnectionModalProps {
  onClose: () => void;
}

export interface CreateConnectionSelectorProps {
  onClose: () => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}
