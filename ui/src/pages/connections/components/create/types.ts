import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import type { FilamentLayoutSearch } from "@/module/schemas";

export interface CreateConnectionModalProps {
  isOpen: boolean;
  connector: FilamentLayoutSearch["connector"];
  connectorKind: FilamentLayoutSearch["connectorKind"];
  connectorSearch: FilamentLayoutSearch["connectorSearch"];
  onClose: () => void;
}

export interface CreateConnectionSelectorProps {
  isOpen: boolean;
  connectorKind: ConnectorKind;
  connectorSearch: string;
  onClose: () => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

export enum CreateConnectionSelectorShelf {
  ALL = "ALL",
  STABLE = "STABLE",
  BETA = "BETA",
  ALPHA = "ALPHA",
}
