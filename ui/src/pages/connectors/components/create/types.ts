import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

export interface CreateConnectionModalProps {
  onClose: () => void;
}

export interface CreateConnectionSelectorProps {
  onClose: () => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

export enum CreateConnectionSelectorShelf {
  ALL = "ALL",
  STABLE = "STABLE",
  BETA = "BETA",
  ALPHA = "ALPHA",
}
