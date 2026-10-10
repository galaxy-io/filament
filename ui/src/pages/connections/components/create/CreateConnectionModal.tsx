import { type FC, useCallback } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import CreateConnectionConfigure from "@/pages/connections/components/create/configure/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connections/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connections/components/create/types";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { useOverlayRecord } from "@/hooks/useOverlayRecord";
import { useOverlaySession } from "@/hooks/useOverlaySession";

const CreateConnectionModal: FC<CreateConnectionModalProps> = ({
  isOpen,
  connector,
  connectorKind,
  connectorSearch,
  onClose,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const session = useOverlaySession(isOpen);
  const shownConnector = useOverlayRecord(isOpen, connector);
  const shownConnectorKind = useOverlayRecord(isOpen, connectorKind);

  const handleConnectorSelect = useCallback(
    (connector: ConnectorSpec) => {
      void updateSearch((prev) => ({
        ...prev,
        connector: connector.aliasTarget || connector.name,
        connectorKind: connector.kind,
        connectorSearch: undefined,
      }));
    },
    [updateSearch],
  );

  const handleBack = useCallback(() => {
    void updateSearch((prev) => ({
      ...prev,
      connector: undefined,
    }));
  }, [updateSearch]);

  if (!session) {
    return null;
  }

  if (shownConnector && shownConnectorKind) {
    return (
      <CreateConnectionConfigure
        key={`${session}:${shownConnectorKind}:${shownConnector}`}
        isOpen={isOpen}
        connector={shownConnector}
        connectorKind={shownConnectorKind}
        onClose={onClose}
        onBack={handleBack}
      />
    );
  }

  return (
    <CreateConnectionSelector
      key={session}
      isOpen={isOpen}
      connectorKind={shownConnectorKind ?? ConnectorKind.UNSPECIFIED}
      connectorSearch={connectorSearch ?? ""}
      onClose={onClose}
      onConnectorSelect={handleConnectorSelect}
    />
  );
};

export default CreateConnectionModal;
