import { useCallback } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import CreateConnectionConfigure from "@/pages/connectors/components/create/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connectors/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connectors/components/create/types";

const CreateConnectionModal = ({ onClose }: CreateConnectionModalProps) => {
  const navigate = useNavigate();
  const { connector, connectorKind } = useSearch({
    from: "__root__",
  });

  const handleConnectorSelect = useCallback(
    (connector: ConnectorSpec) => {
      void navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connector: connector.name,
          connectorKind: connector.kind,
        }),
      });
    },
    [navigate],
  );

  const handleBack = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        connector: undefined,
      }),
    });
  }, [navigate]);

  if (connector && connectorKind) {
    return <CreateConnectionConfigure onClose={onClose} onBack={handleBack} />;
  }

  return <CreateConnectionSelector onClose={onClose} onConnectorSelect={handleConnectorSelect} />;
};

export default CreateConnectionModal;
