import { type FC, useCallback } from "react";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import CreateConnectionConfigure from "@/pages/connections/components/create/configure/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connections/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connections/components/create/types";

import { useFilamentLayoutSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

const CreateConnectionModal: FC<CreateConnectionModalProps> = ({ onClose }) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const { connector, connectorKind } = useFilamentLayoutSearch();

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

  if (connector && connectorKind) {
    return (
      <CreateConnectionConfigure
        key={`${connectorKind}:${connector}`}
        onClose={onClose}
        onBack={handleBack}
      />
    );
  }

  return <CreateConnectionSelector onClose={onClose} onConnectorSelect={handleConnectorSelect} />;
};

export default CreateConnectionModal;
