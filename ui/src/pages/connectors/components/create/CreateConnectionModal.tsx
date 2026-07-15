import { useCallback, useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import CreateConnectionConfigure from "@/pages/connectors/components/create/configure/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connectors/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connectors/components/create/types";

const CreateConnectionModal = ({ onClose }: CreateConnectionModalProps) => {
  const navigate = useNavigate();
  const { connector: connectorParam } = useSearch({ from: "__root__" });

  const { data } = useListConnectorsQuery();

  const selectedConnector = useMemo(() => {
    if (!connectorParam || !data?.connectors) return null;
    return data.connectors.find((c) => c.name === connectorParam) ?? null;
  }, [connectorParam, data?.connectors]);

  const handleConnectorSelect = useCallback(
    (connector: ConnectorSpec) => {
      void navigate({
        to: ".",
        search: (prev) => ({ ...prev, connector: connector.name }),
      });
    },
    [navigate],
  );

  const handleBack = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { connector: _, ...rest } = prev;
        return rest;
      },
    });
  }, [navigate]);

  if (selectedConnector) {
    return (
      <CreateConnectionConfigure
        connector={selectedConnector}
        onClose={onClose}
        onBack={handleBack}
      />
    );
  }

  return <CreateConnectionSelector onClose={onClose} onConnectorSelect={handleConnectorSelect} />;
};

export default CreateConnectionModal;
