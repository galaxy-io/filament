import { useCallback, useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import CreateConnectionConfigure from "@/pages/connectors/components/create/configure/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connectors/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connectors/components/create/types";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

const CreateConnectionModal = ({ onClose }: CreateConnectionModalProps) => {
  const navigate = useNavigate();
  const { connector: connectorParam, connectorKind: connectorKindParam } = useSearch({
    from: "__root__",
  });

  const { data } = useListConnectorsQuery();

  // A connector name can exist as both a source and a sink (e.g. postgres), so
  // the kind must disambiguate which spec the configure step uses
  const selectedConnector = useMemo(() => {
    if (!connectorParam || !data?.connectors) return null;
    return (
      data.connectors.find(
        (c) =>
          c.name === connectorParam &&
          (connectorKindParam === undefined || c.kind === connectorKindParam),
      ) ?? null
    );
  }, [connectorParam, connectorKindParam, data?.connectors]);

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
