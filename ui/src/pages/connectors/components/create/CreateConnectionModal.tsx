import { useCallback } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import CreateConnectionConfigure from "@/pages/connectors/components/create/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connectors/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connectors/components/create/types";
import { CONNECTOR_KIND_TO_PARAM_MAP } from "@/pages/connectors/constants";

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
          connectorKind: CONNECTOR_KIND_TO_PARAM_MAP[connector.kind],
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
